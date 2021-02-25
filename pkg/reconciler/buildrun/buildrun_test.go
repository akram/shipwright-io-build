// Copyright The Shipwright Contributors
//
// SPDX-License-Identifier: Apache-2.0

package buildrun_test

import (
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/shipwright-io/build/pkg/apis"
	build "github.com/shipwright-io/build/pkg/apis/build/v1alpha1"
	"github.com/shipwright-io/build/pkg/config"
	"github.com/shipwright-io/build/pkg/controller/fakes"
	"github.com/shipwright-io/build/pkg/ctxlog"
	buildrunctl "github.com/shipwright-io/build/pkg/reconciler/buildrun"
	"github.com/shipwright-io/build/test"
)

var _ = Describe("Reconcile BuildRun", func() {
	var (
		manager                                                *fakes.FakeManager
		reconciler                                             reconcile.Reconciler
		taskRunRequest                                         reconcile.Request
		client                                                 *fakes.FakeClient
		ctl                                                    test.Catalog
		buildSample                                            *build.Build
		buildRunSample                                         *build.BuildRun
		taskRunSample                                          *v1beta1.TaskRun
		statusWriter                                           *fakes.FakeStatusWriter
		taskRunName, buildRunName, buildName, strategyName, ns string
	)

	// returns a reconcile.Request based on an resource name and namespace
	newReconcileRequest := func(name string, ns string) reconcile.Request {
		return reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: ns,
			},
		}
	}

	// Basic stubs that simulate the output of all client calls in the Reconciler logic.
	// This applies only for a Build and BuildRun client get.
	getClientStub := func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
		switch object := object.(type) {
		case *build.Build:
			buildSample.DeepCopyInto(object)
			return nil
		case *build.BuildRun:
			buildRunSample.DeepCopyInto(object)
			return nil
		case *v1beta1.TaskRun:
			taskRunSample.DeepCopyInto(object)
			return nil
		}
		return k8serrors.NewNotFound(schema.GroupResource{}, nn.Name)
	}

	BeforeEach(func() {
		strategyName = "foobar-strategy"
		buildName = "foobar-build"
		buildRunName = "foobar-buildrun"
		taskRunName = "foobar-buildrun-p8nts"
		ns = "default"

		// ensure resources are added to the Scheme
		// via the manager and initialize the fake Manager
		apis.AddToScheme(scheme.Scheme)
		manager = &fakes.FakeManager{}
		manager.GetSchemeReturns(scheme.Scheme)

		// initialize the fake client and let the
		// client know on the stubs when get calls are executed
		client = &fakes.FakeClient{}
		client.GetCalls(getClientStub)

		// initialize the fake status writer, this is needed for
		// all status updates during reconciliation
		statusWriter = &fakes.FakeStatusWriter{}
		client.StatusCalls(func() crc.StatusWriter { return statusWriter })
		manager.GetClientReturns(client)

		// init the Build resource, this never change throughout this test suite
		buildSample = ctl.DefaultBuild(buildName, strategyName, build.ClusterBuildStrategyKind)

	})

	// JustBeforeEach will always execute just before the It() specs,
	// this ensures that overrides on the BuildRun resource can happen under each
	// Context() BeforeEach() block
	JustBeforeEach(func() {
		// l := ctxlog.NewLogger("buildrun-controller-test")
		// testCtx := ctxlog.NewParentContext(l)
		testCtx := ctxlog.NewContext(context.TODO(), "fake-logger")
		reconciler = buildrunctl.NewReconciler(testCtx, config.NewDefaultConfig(), manager, controllerutil.SetControllerReference)
	})

	Describe("Reconciling", func() {
		Context("from an existing TaskRun with Conditions", func() {
			BeforeEach(func() {

				// Generate a new Reconcile Request using the existing TaskRun name and namespace
				taskRunRequest = newReconcileRequest(taskRunName, ns)

				// initialize a BuildRun, we need this to fake the existence of a BuildRun
				buildRunSample = ctl.DefaultBuildRun(buildRunName, buildName)
			})

			It("updates the BuildRun status with a SUCCEEDED reason", func() {

				taskRunSample = ctl.DefaultTaskRunWithStatus(taskRunName, buildRunName, ns, corev1.ConditionTrue, "Succeeded")

				// Stub that asserts the BuildRun status fields when
				// Status updates for a BuildRun take place
				statusCall := ctl.StubBuildRunStatus(
					"Succeeded",
					&taskRunName,
					build.Condition{
						Type:   build.Succeeded,
						Reason: "Succeeded",
						Status: corev1.ConditionTrue,
					},
					corev1.ConditionTrue,
					buildSample.Spec,
					false,
				)
				statusWriter.UpdateCalls(statusCall)
				taskRunSample.Spec.ServiceAccountName = "toto"
				result, err := reconciler.Reconcile(taskRunRequest)
				Expect(err).ToNot(HaveOccurred())
				Expect(reconcile.Result{}).To(Equal(result))
				Expect(client.GetCallCount()).To(Equal(2))
				//Expect(client.StatusCallCount()).To(Equal(1))
				Expect(taskRunSample.Spec.ServiceAccountName).NotTo(Equal(nil))
				Expect(taskRunSample.Spec.ServiceAccountName).To(Equal(buildRunSample.Status.ServiceAccountName))

			})
		})
	})
})
