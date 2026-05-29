/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// This file contains unit tests for the ReplicationController controller
// wrapper (ReplicationManager) and the RC<->RS conversion adapters defined in
// conversion.go. The ReplicationManager is intentionally a thin wrapper around
// the shared ReplicaSetController, with a conversion layer that treats a
// ReplicationController as if it were an older API version of a ReplicaSet.
//
// Testing strategy:
//   - No informers are started: listers are populated by adding objects to the
//     underlying indexer directly. This keeps every test fully synchronous, with
//     no sleeping and no WaitForCacheSync.
//   - Each subtest constructs its own fake clientset and informer factory so
//     there is no shared mutable state between cases (safe under -race and the
//     informer cache-mutation detector).
//   - The idiomatic, repo-native fake clientset
//     (k8s.io/client-go/kubernetes/fake.NewSimpleClientset) is used in place of
//     controller-runtime's fake client builder.
package replication

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"unsafe"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apps "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	appsv1apply "k8s.io/client-go/applyconfigurations/apps/v1"
	appsv1autoscaling "k8s.io/client-go/applyconfigurations/autoscaling/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	appsv1client "k8s.io/client-go/kubernetes/typed/apps/v1"
	core "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"

	_ "k8s.io/kubernetes/pkg/apis/apps/install"
	_ "k8s.io/kubernetes/pkg/apis/core/install"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/controller/replicaset"
)

// newTestRC constructs a deterministic *v1.ReplicationController for tests.
//
// IMPORTANT: Spec.Replicas MUST be non-nil because the RC->RS conversion path
// (Convert_v1_ReplicationController_To_apps_ReplicaSet) operates on the
// replica count; all fixtures therefore use ptr.To[int32](N).
func newTestRC(name, namespace string, replicas int32, labelMap map[string]string) *v1.ReplicationController {
	// Copy the provided labels into a fresh map so callers can never observe
	// shared mutable state between fixtures.
	rcLabels := map[string]string{}
	for k, v := range labelMap {
		rcLabels[k] = v
	}
	return &v1.ReplicationController{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ReplicationController",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			UID:             types.UID("uid-" + name),
			ResourceVersion: "1",
			Labels:          rcLabels,
			Annotations:     map[string]string{"app.kubernetes.io/managed-by": "test"},
		},
		Spec: v1.ReplicationControllerSpec{
			Replicas: ptr.To[int32](replicas),
			Selector: rcLabels,
			Template: &v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: rcLabels,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{Name: "container", Image: "nginx:1.21"},
					},
					RestartPolicy: v1.RestartPolicyAlways,
					DNSPolicy:     v1.DNSClusterFirst,
				},
			},
		},
		Status: v1.ReplicationControllerStatus{
			Replicas:             replicas,
			FullyLabeledReplicas: replicas,
			ReadyReplicas:        replicas,
			AvailableReplicas:    replicas,
			ObservedGeneration:   1,
		},
	}
}

// newTestRS constructs a deterministic *apps.ReplicaSet matching the shape of
// newTestRC, for round-trip and round-trip-delegation tests.
func newTestRS(name, namespace string, replicas int32, labelMap map[string]string) *apps.ReplicaSet {
	rsLabels := map[string]string{}
	for k, v := range labelMap {
		rsLabels[k] = v
	}
	return &apps.ReplicaSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "ReplicaSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			UID:             types.UID("uid-" + name),
			ResourceVersion: "1",
			Labels:          rsLabels,
		},
		Spec: apps.ReplicaSetSpec{
			Replicas: ptr.To[int32](replicas),
			Selector: &metav1.LabelSelector{MatchLabels: rsLabels},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: rsLabels,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{Name: "container", Image: "nginx:1.21"},
					},
					RestartPolicy: v1.RestartPolicyAlways,
					DNSPolicy:     v1.DNSClusterFirst,
				},
			},
		},
	}
}

// readUnexportedField returns the reflect.Value of an unexported field of s
// (navigated by fieldPath) in a form that is readable via Interface()/Int().
//
// ReplicationManager embeds replicaset.ReplicaSetController, whose burstReplicas
// and controllerFeatures fields are unexported in package replicaset. Go's
// visibility rules apply across packages even through embedding, so the only
// way to assert on these fields from package replication is via reflection plus
// unsafe.Pointer to clear the read-only flag.
func readUnexportedField(t *testing.T, s interface{}, fieldPath ...string) reflect.Value {
	t.Helper()
	val := reflect.ValueOf(s)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	for _, name := range fieldPath {
		val = val.FieldByName(name)
		if !val.IsValid() {
			t.Fatalf("field %q not found while reading unexported field path %v", name, fieldPath)
		}
		// reflect.NewAt(type, addr).Elem() yields a Value without the
		// read-only flag, allowing Interface()/Int() on unexported fields.
		if val.CanAddr() {
			val = reflect.NewAt(val.Type(), unsafe.Pointer(val.UnsafeAddr())).Elem()
		}
	}
	return val
}

// addCall records a single OnAdd invocation.
type addCall struct {
	obj             interface{}
	isInInitialList bool
}

// updateCall records a single OnUpdate invocation.
type updateCall struct {
	oldObj interface{}
	newObj interface{}
}

// fakeResourceEventHandler is a thread-safe recorder implementing
// cache.ResourceEventHandler. It is used to verify that conversionEventHandler
// dispatches the correctly-converted *apps.ReplicaSet (or tombstone wrapping
// one) to the downstream handler for OnAdd/OnUpdate/OnDelete events.
type fakeResourceEventHandler struct {
	mu          sync.Mutex
	addCalls    []addCall
	updateCalls []updateCall
	deleteCalls []interface{}
}

func (f *fakeResourceEventHandler) OnAdd(obj interface{}, isInInitialList bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.addCalls = append(f.addCalls, addCall{obj: obj, isInInitialList: isInInitialList})
}

func (f *fakeResourceEventHandler) OnUpdate(oldObj, newObj interface{}) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updateCalls = append(f.updateCalls, updateCall{oldObj: oldObj, newObj: newObj})
}

func (f *fakeResourceEventHandler) OnDelete(obj interface{}) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleteCalls = append(f.deleteCalls, obj)
}

// snapshot returns copies of the recorded calls for thread-safe assertions.
func (f *fakeResourceEventHandler) snapshot() (adds []addCall, updates []updateCall, deletes []interface{}) {
	f.mu.Lock()
	defer f.mu.Unlock()
	adds = append(adds, f.addCalls...)
	updates = append(updates, f.updateCalls...)
	deletes = append(deletes, f.deleteCalls...)
	return adds, updates, deletes
}

// TestNewReplicationManager verifies that the ReplicationManager constructor
// wires the embedded ReplicaSetController with the ReplicationController GVK,
// the requested burstReplicas, and the RC-specific feature toggle
// (EnableStatusTerminatingReplicas=false, because the RC API has no
// .status.terminatingReplicas field).
func TestNewReplicationManager(t *testing.T) {
	tests := []struct {
		name          string
		burstReplicas int
	}{
		{
			name:          "default BurstReplicas",
			burstReplicas: BurstReplicas,
		},
		{
			name:          "small burst",
			burstReplicas: 10,
		},
		{
			name:          "zero burst",
			burstReplicas: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			client := fake.NewSimpleClientset()
			factory := informers.NewSharedInformerFactory(client, controller.NoResyncPeriodFunc())

			rm := NewReplicationManager(
				ctx,
				factory.Core().V1().Pods(),
				factory.Core().V1().ReplicationControllers(),
				client,
				tc.burstReplicas,
			)
			require.NotNil(t, rm, "NewReplicationManager returned nil")

			// GroupVersionKind is exported (promoted from the embedded
			// schema.GroupVersionKind) so it is directly accessible.
			expectedGVK := v1.SchemeGroupVersion.WithKind("ReplicationController")
			assert.Equal(t, expectedGVK, rm.GroupVersionKind, "GroupVersionKind mismatch")
			assert.Equal(t, "ReplicationController", rm.Kind, "Kind should be ReplicationController")
			assert.Equal(t, "", rm.Group, "Group should be empty for core/v1")
			assert.Equal(t, "v1", rm.Version, "Version should be v1")

			// burstReplicas is unexported in package replicaset; read it via
			// reflection+unsafe from the embedded ReplicaSetController.
			burstVal := readUnexportedField(t, &rm.ReplicaSetController, "burstReplicas")
			require.True(t, burstVal.IsValid(), "burstReplicas field not found")
			assert.Equal(t, tc.burstReplicas, int(burstVal.Int()), "burstReplicas mismatch")

			// controllerFeatures is unexported as well; assert the RC-specific
			// toggle is disabled.
			featuresVal := readUnexportedField(t, &rm.ReplicaSetController, "controllerFeatures")
			require.True(t, featuresVal.IsValid(), "controllerFeatures field not found")
			features, ok := featuresVal.Interface().(replicaset.ReplicaSetControllerFeatures)
			require.True(t, ok, "controllerFeatures type assertion failed; got %T", featuresVal.Interface())
			assert.False(t, features.EnableStatusTerminatingReplicas,
				"ReplicationController controller must NOT enable status terminating replicas")

			// Controller-name contract ("replication_controller"):
			//
			// NewReplicationManager passes "replication_controller" as the
			// metricOwnerName positional argument to replicaset.NewBaseController
			// (see replication_controller.go). However, NewBaseController does NOT
			// retain metricOwnerName in any exported or unexported field of the
			// returned ReplicaSetController: the parameter appears only in the
			// NewBaseController signature in replica_set.go and is never assigned.
			// The adjacent queueName argument ("replicationmanager") IS consumed,
			// but only as the workqueue's metrics name, which the no-op metrics
			// provider used in unit tests discards. Neither string is therefore
			// reliably observable on the constructed object, so asserting
			// "replication_controller" directly would require a production change,
			// which is out of scope for this test-only change.
			//
			// Closest enforceable assertion: confirm the embedded controller's
			// workqueue was constructed by NewBaseController. A non-nil queue
			// proves the full (gvk, metricOwnerName, queueName, ...) argument tuple
			// was accepted and wired; combined with the GroupVersionKind assertion
			// above, this is the strongest construction-identity guarantee
			// observable from the returned *ReplicationManager.
			queueVal := readUnexportedField(t, &rm.ReplicaSetController, "queue")
			require.True(t, queueVal.IsValid(), "queue field not found on embedded ReplicaSetController")
			assert.False(t, queueVal.IsNil(), "NewBaseController must construct the controller workqueue")
		})
	}
}

// TestConvertRCtoRSAndBack exercises the convertRCtoRS / convertRStoRC pair and
// asserts that a full RC -> RS -> RC round-trip preserves identity metadata,
// labels, annotations, finalizers, owner references, replica count, selector,
// and observedGeneration.
//
// NOTE: apps.ReplicaSet.Spec.Replicas and v1.ReplicationControllerSpec.Replicas
// are both *int32, so replica comparisons dereference the pointer after a
// non-nil guard.
func TestConvertRCtoRSAndBack(t *testing.T) {
	trueVal := true
	tests := []struct {
		name string
		rc   *v1.ReplicationController
	}{
		{
			name: "basic RC with selector and labels",
			rc:   newTestRC("rc-1", "ns-1", 3, map[string]string{"app": "test"}),
		},
		{
			name: "RC with owner references and observedGeneration",
			rc: func() *v1.ReplicationController {
				rc := newTestRC("rc-2", "ns-2", 5, map[string]string{"app": "ownerref"})
				rc.OwnerReferences = []metav1.OwnerReference{
					{
						APIVersion:         "apps/v1",
						Kind:               "Deployment",
						Name:               "parent-deployment",
						UID:                types.UID("deployment-uid"),
						Controller:         &trueVal,
						BlockOwnerDeletion: &trueVal,
					},
				}
				rc.Status.ObservedGeneration = 42
				rc.Generation = 42
				return rc
			}(),
		},
		{
			name: "RC with zero replicas",
			rc:   newTestRC("rc-zero", "ns-zero", 0, map[string]string{"app": "zero"}),
		},
		{
			name: "RC with finalizers and multiple labels",
			rc: func() *v1.ReplicationController {
				rc := newTestRC("rc-3", "ns-3", 2, map[string]string{"app": "multi", "tier": "frontend", "env": "prod"})
				rc.Finalizers = []string{"example.com/finalizer-1", "example.com/finalizer-2"}
				return rc
			}(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Step 1: RC -> RS (nil out parameter => function allocates).
			rs, err := convertRCtoRS(tc.rc.DeepCopy(), nil)
			require.NoError(t, err, "convertRCtoRS returned error")
			require.NotNil(t, rs, "convertRCtoRS returned nil RS")

			assert.Equal(t, tc.rc.Name, rs.Name, "Name mismatch")
			assert.Equal(t, tc.rc.Namespace, rs.Namespace, "Namespace mismatch")
			assert.Equal(t, tc.rc.UID, rs.UID, "UID mismatch")
			assert.Equal(t, tc.rc.ResourceVersion, rs.ResourceVersion, "ResourceVersion mismatch")
			require.NotNil(t, rs.Spec.Replicas, "RS Spec.Replicas should be non-nil after conversion")
			assert.Equal(t, *tc.rc.Spec.Replicas, *rs.Spec.Replicas, "Replicas mismatch")
			assert.Equal(t, tc.rc.Labels, rs.Labels, "Labels mismatch")
			assert.Equal(t, tc.rc.Status.ObservedGeneration, rs.Status.ObservedGeneration, "ObservedGeneration mismatch")
			assert.Equal(t, tc.rc.OwnerReferences, rs.OwnerReferences, "OwnerReferences mismatch")
			// Annotations and finalizers are part of ObjectMeta and must survive
			// the conversion (the AAP requires labels AND annotations preservation;
			// the "RC with finalizers" fixture row also exercises finalizers).
			assert.Equal(t, tc.rc.Annotations, rs.Annotations, "Annotations mismatch")
			assert.Equal(t, tc.rc.Finalizers, rs.Finalizers, "Finalizers mismatch")

			// Step 2: RS -> RC (round-trip).
			rc2, err := convertRStoRC(rs)
			require.NoError(t, err, "convertRStoRC returned error")
			require.NotNil(t, rc2, "convertRStoRC returned nil RC")

			assert.Equal(t, tc.rc.Name, rc2.Name, "Round-trip Name mismatch")
			assert.Equal(t, tc.rc.Namespace, rc2.Namespace, "Round-trip Namespace mismatch")
			assert.Equal(t, tc.rc.UID, rc2.UID, "Round-trip UID mismatch")
			assert.Equal(t, tc.rc.ResourceVersion, rc2.ResourceVersion, "Round-trip ResourceVersion mismatch")
			require.NotNil(t, rc2.Spec.Replicas, "Round-trip Spec.Replicas is nil")
			assert.Equal(t, *tc.rc.Spec.Replicas, *rc2.Spec.Replicas, "Round-trip Replicas mismatch")
			assert.Equal(t, tc.rc.Labels, rc2.Labels, "Round-trip Labels mismatch")
			assert.Equal(t, tc.rc.Spec.Selector, rc2.Spec.Selector, "Round-trip Selector mismatch")
			assert.Equal(t, tc.rc.Status.ObservedGeneration, rc2.Status.ObservedGeneration, "Round-trip ObservedGeneration mismatch")
			assert.Equal(t, tc.rc.OwnerReferences, rc2.OwnerReferences, "Round-trip OwnerReferences mismatch")
			assert.Equal(t, tc.rc.Annotations, rc2.Annotations, "Round-trip Annotations mismatch")
			assert.Equal(t, tc.rc.Finalizers, rc2.Finalizers, "Round-trip Finalizers mismatch")

			// Surface any selector difference with a readable diff.
			if diff := cmp.Diff(tc.rc.Spec.Selector, rc2.Spec.Selector); diff != "" {
				t.Errorf("Selector mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestConvertRCtoRSReusesOut verifies that passing a non-nil out RS reuses the
// provided struct rather than allocating a new one.
func TestConvertRCtoRSReusesOut(t *testing.T) {
	rc := newTestRC("rc-reuse", "ns", 2, map[string]string{"app": "reuse"})
	existing := &apps.ReplicaSet{}

	result, err := convertRCtoRS(rc, existing)
	require.NoError(t, err)
	assert.Same(t, existing, result, "convertRCtoRS should reuse the provided out RS")
	assert.Equal(t, rc.Name, result.Name, "result should be populated from RC")
}

// TestConvertSlice verifies that convertSlice converts every RC in the input
// slice to an RS, preserving order, and that empty/nil inputs yield an empty
// (non-error) result.
func TestConvertSlice(t *testing.T) {
	tests := []struct {
		name      string
		input     []*v1.ReplicationController
		wantLen   int
		wantNames []string
	}{
		{
			name:      "empty slice",
			input:     []*v1.ReplicationController{},
			wantLen:   0,
			wantNames: []string{},
		},
		{
			name:      "nil slice",
			input:     nil,
			wantLen:   0,
			wantNames: []string{},
		},
		{
			name: "three RCs",
			input: []*v1.ReplicationController{
				newTestRC("rc-a", "ns", 1, map[string]string{"app": "a"}),
				newTestRC("rc-b", "ns", 2, map[string]string{"app": "b"}),
				newTestRC("rc-c", "ns", 3, map[string]string{"app": "c"}),
			},
			wantLen:   3,
			wantNames: []string{"rc-a", "rc-b", "rc-c"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := convertSlice(tc.input)
			require.NoError(t, err, "convertSlice returned error")
			require.Len(t, result, tc.wantLen, "convertSlice returned wrong length")
			for i, want := range tc.wantNames {
				assert.Equal(t, want, result[i].Name, "Name mismatch at index %d", i)
			}
		})
	}
}

// TestConvertList verifies that convertList converts a ReplicationControllerList
// into a ReplicaSetList, preserving item order and replica counts. (RS replica
// counts are *int32 and are dereferenced for comparison.)
func TestConvertList(t *testing.T) {
	rcList := &v1.ReplicationControllerList{
		ListMeta: metav1.ListMeta{
			ResourceVersion: "100",
			Continue:        "continue-token",
		},
		Items: []v1.ReplicationController{
			*newTestRC("rc-1", "ns", 2, map[string]string{"app": "a"}),
			*newTestRC("rc-2", "ns", 4, map[string]string{"app": "b"}),
		},
	}

	rsList, err := convertList(rcList)
	require.NoError(t, err, "convertList returned error")
	require.NotNil(t, rsList, "convertList returned nil")
	require.Len(t, rsList.Items, 2, "Item count mismatch")
	assert.Equal(t, "rc-1", rsList.Items[0].Name)
	assert.Equal(t, "rc-2", rsList.Items[1].Name)
	require.NotNil(t, rsList.Items[0].Spec.Replicas)
	require.NotNil(t, rsList.Items[1].Spec.Replicas)
	assert.Equal(t, int32(2), *rsList.Items[0].Spec.Replicas)
	assert.Equal(t, int32(4), *rsList.Items[1].Spec.Replicas)
}

// TestConvertCall verifies the convertCall helper, which converts the RS
// argument to an RC, invokes the provided function, and converts the result
// back to an RS. Both the success and error paths are covered.
func TestConvertCall(t *testing.T) {
	rs := newTestRS("rs-call", "ns", 3, map[string]string{"app": "call"})

	t.Run("successful call", func(t *testing.T) {
		called := false
		var receivedRC *v1.ReplicationController
		fn := func(rc *v1.ReplicationController) (*v1.ReplicationController, error) {
			called = true
			receivedRC = rc
			return rc, nil
		}

		result, err := convertCall(fn, rs.DeepCopy())
		require.NoError(t, err)
		assert.True(t, called, "fn should have been called")
		require.NotNil(t, receivedRC, "fn should have received an RC")
		assert.Equal(t, rs.Name, receivedRC.Name, "received RC name mismatch")
		require.NotNil(t, result, "result RS should be non-nil")
		assert.Equal(t, rs.Name, result.Name, "result RS name mismatch")
	})

	t.Run("fn returns error", func(t *testing.T) {
		fn := func(rc *v1.ReplicationController) (*v1.ReplicationController, error) {
			return nil, apierrors.NewInternalError(reflectFakeError())
		}
		result, err := convertCall(fn, rs.DeepCopy())
		assert.Nil(t, result)
		require.Error(t, err)
	})
}

// reflectFakeError returns the testify sentinel error, used where the exact
// error type is irrelevant.
func reflectFakeError() error {
	return assert.AnError
}

// TestInformerAdapter verifies that informerAdapter.Informer() and
// informerAdapter.Lister() return the conversion-wrapping types.
func TestInformerAdapter(t *testing.T) {
	// newAdapter builds a fresh informerAdapter (with its own fake clientset and
	// informer factory) so each subtest owns isolated mutable state and is
	// independently runnable, per the no-shared-mutable-state directive.
	newAdapter := func() informerAdapter {
		client := fake.NewSimpleClientset()
		factory := informers.NewSharedInformerFactory(client, controller.NoResyncPeriodFunc())
		return informerAdapter{rcInformer: factory.Core().V1().ReplicationControllers()}
	}

	t.Run("Informer returns non-nil conversionInformer", func(t *testing.T) {
		adapter := newAdapter()
		inf := adapter.Informer()
		require.NotNil(t, inf)
		_, ok := inf.(conversionInformer)
		assert.True(t, ok, "Informer() should return conversionInformer; got %T", inf)
	})

	t.Run("Lister returns non-nil conversionLister", func(t *testing.T) {
		adapter := newAdapter()
		lister := adapter.Lister()
		require.NotNil(t, lister)
		_, ok := lister.(conversionLister)
		assert.True(t, ok, "Lister() should return conversionLister; got %T", lister)
	})
}

// TestConversionInformerAddEventHandler verifies that both event-handler
// registration methods wrap the provided handler and return a registration
// handle without error (the underlying informer is never started).
func TestConversionInformerAddEventHandler(t *testing.T) {
	// newConvInformer builds a fresh conversionInformer wrapping its own informer
	// so the two subtests never register handlers on a shared informer (which
	// would be shared mutable state across cases).
	newConvInformer := func() conversionInformer {
		client := fake.NewSimpleClientset()
		factory := informers.NewSharedInformerFactory(client, controller.NoResyncPeriodFunc())
		return conversionInformer{factory.Core().V1().ReplicationControllers().Informer()}
	}

	t.Run("AddEventHandler returns registration", func(t *testing.T) {
		convInformer := newConvInformer()
		handler := &fakeResourceEventHandler{}
		reg, err := convInformer.AddEventHandler(handler)
		require.NoError(t, err)
		assert.NotNil(t, reg, "AddEventHandler should return registration handle")
	})

	t.Run("AddEventHandlerWithResyncPeriod returns registration", func(t *testing.T) {
		convInformer := newConvInformer()
		handler := &fakeResourceEventHandler{}
		reg, err := convInformer.AddEventHandlerWithResyncPeriod(handler, 0)
		require.NoError(t, err)
		assert.NotNil(t, reg, "AddEventHandlerWithResyncPeriod should return registration handle")
	})
}

// TestConversionLister exercises conversionLister and conversionNamespaceLister:
// cluster-wide List (with and without a selector), namespace-scoped List/Get
// (including the NotFound path), and GetPodReplicaSets for the matching,
// no-matching-RC, and no-labels cases.
//
// The informer is never started; the lister is populated by adding RCs to the
// underlying indexer directly.
func TestConversionLister(t *testing.T) {
	// newPopulatedLister builds a fresh conversionLister backed by its own
	// informer indexer, pre-populated with the same three RCs, so each subtest
	// owns isolated state and is independently runnable. The informer is never
	// started; objects are added to the indexer directly.
	newPopulatedLister := func(t *testing.T) conversionLister {
		t.Helper()
		client := fake.NewSimpleClientset()
		factory := informers.NewSharedInformerFactory(client, controller.NoResyncPeriodFunc())
		rcInformer := factory.Core().V1().ReplicationControllers()
		rcs := []*v1.ReplicationController{
			newTestRC("rc-a", "ns-1", 1, map[string]string{"app": "a"}),
			newTestRC("rc-b", "ns-1", 2, map[string]string{"app": "b"}),
			newTestRC("rc-c", "ns-2", 3, map[string]string{"app": "a"}),
		}
		for _, rc := range rcs {
			require.NoError(t, rcInformer.Informer().GetIndexer().Add(rc))
		}
		return conversionLister{rcLister: rcInformer.Lister()}
	}

	t.Run("List all", func(t *testing.T) {
		lister := newPopulatedLister(t)
		got, err := lister.List(labels.Everything())
		require.NoError(t, err)
		assert.Len(t, got, 3, "List(everything) should return all 3 RCs")
		gotNames := map[string]bool{}
		for _, rs := range got {
			gotNames[rs.Name] = true
		}
		assert.True(t, gotNames["rc-a"])
		assert.True(t, gotNames["rc-b"])
		assert.True(t, gotNames["rc-c"])
	})

	t.Run("List with selector filter", func(t *testing.T) {
		lister := newPopulatedLister(t)
		got, err := lister.List(labels.SelectorFromSet(labels.Set{"app": "a"}))
		require.NoError(t, err)
		assert.Len(t, got, 2, "should match rc-a and rc-c (both app=a)")
	})

	t.Run("Namespace lister List", func(t *testing.T) {
		lister := newPopulatedLister(t)
		nsLister := lister.ReplicaSets("ns-1")
		got, err := nsLister.List(labels.Everything())
		require.NoError(t, err)
		assert.Len(t, got, 2, "ns-1 should have 2 RCs")
	})

	t.Run("Namespace lister Get existing", func(t *testing.T) {
		lister := newPopulatedLister(t)
		nsLister := lister.ReplicaSets("ns-1")
		got, err := nsLister.Get("rc-a")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, "rc-a", got.Name)
		require.NotNil(t, got.Spec.Replicas)
		assert.Equal(t, int32(1), *got.Spec.Replicas)
	})

	t.Run("Namespace lister Get missing returns NotFound", func(t *testing.T) {
		lister := newPopulatedLister(t)
		nsLister := lister.ReplicaSets("ns-1")
		got, err := nsLister.Get("missing")
		assert.Nil(t, got)
		require.Error(t, err)
		assert.True(t, apierrors.IsNotFound(err), "expected NotFound error, got: %v", err)
	})

	t.Run("GetPodReplicaSets matching pod", func(t *testing.T) {
		lister := newPopulatedLister(t)
		pod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-a",
				Namespace: "ns-1",
				Labels:    map[string]string{"app": "a"},
			},
		}
		got, err := lister.GetPodReplicaSets(pod)
		require.NoError(t, err)
		require.Len(t, got, 1, "should match rc-a in ns-1 only")
		assert.Equal(t, "rc-a", got[0].Name)
	})

	t.Run("GetPodReplicaSets no matching RC returns error", func(t *testing.T) {
		lister := newPopulatedLister(t)
		pod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-z",
				Namespace: "ns-1",
				Labels:    map[string]string{"app": "non-existent"},
			},
		}
		// Per replicationcontroller_expansion.go, GetPodControllers (and thus
		// GetPodReplicaSets) returns an error when no RC matches, NOT an empty
		// slice.
		got, err := lister.GetPodReplicaSets(pod)
		assert.Nil(t, got)
		require.Error(t, err, "expected error when no RC matches pod labels")
		assert.Contains(t, err.Error(), "could not find controller for pod")
	})

	t.Run("GetPodReplicaSets pod with no labels returns error", func(t *testing.T) {
		lister := newPopulatedLister(t)
		pod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-no-labels",
				Namespace: "ns-1",
			},
		}
		got, err := lister.GetPodReplicaSets(pod)
		assert.Nil(t, got)
		require.Error(t, err, "expected error when pod has no labels")
		assert.Contains(t, err.Error(), "no controllers found")
	})
}

// TestConversionEventHandlerOnAdd verifies that OnAdd converts the RC to an RS
// and dispatches it (preserving the isInInitialList flag) to the downstream
// handler.
func TestConversionEventHandlerOnAdd(t *testing.T) {
	rc := newTestRC("rc-add", "ns", 2, map[string]string{"app": "add"})
	tests := []struct {
		name          string
		isInitialList bool
	}{
		{name: "OnAdd not initial", isInitialList: false},
		{name: "OnAdd initial list", isInitialList: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fakeHandler := &fakeResourceEventHandler{}
			convHandler := conversionEventHandler{handler: fakeHandler}

			convHandler.OnAdd(rc.DeepCopy(), tc.isInitialList)

			adds, _, _ := fakeHandler.snapshot()
			require.Len(t, adds, 1, "expected exactly one OnAdd call")
			assert.Equal(t, tc.isInitialList, adds[0].isInInitialList)
			rs, ok := adds[0].obj.(*apps.ReplicaSet)
			require.True(t, ok, "OnAdd should receive *apps.ReplicaSet; got %T", adds[0].obj)
			assert.Equal(t, "rc-add", rs.Name)
			require.NotNil(t, rs.Spec.Replicas)
			assert.Equal(t, int32(2), *rs.Spec.Replicas)
		})
	}
}

// TestConversionEventHandlerOnUpdate verifies that OnUpdate converts both the
// old and new RCs to RSs and dispatches them to the downstream handler.
func TestConversionEventHandlerOnUpdate(t *testing.T) {
	oldRC := newTestRC("rc-up", "ns", 2, map[string]string{"app": "up"})
	newRC := newTestRC("rc-up", "ns", 3, map[string]string{"app": "up"})
	newRC.ResourceVersion = "2"

	fakeHandler := &fakeResourceEventHandler{}
	convHandler := conversionEventHandler{handler: fakeHandler}

	convHandler.OnUpdate(oldRC.DeepCopy(), newRC.DeepCopy())

	_, updates, _ := fakeHandler.snapshot()
	require.Len(t, updates, 1, "expected exactly one OnUpdate call")
	oldRS, ok := updates[0].oldObj.(*apps.ReplicaSet)
	require.True(t, ok, "oldObj should be *apps.ReplicaSet; got %T", updates[0].oldObj)
	newRS, ok := updates[0].newObj.(*apps.ReplicaSet)
	require.True(t, ok, "newObj should be *apps.ReplicaSet; got %T", updates[0].newObj)
	require.NotNil(t, oldRS.Spec.Replicas)
	require.NotNil(t, newRS.Spec.Replicas)
	assert.Equal(t, int32(2), *oldRS.Spec.Replicas)
	assert.Equal(t, int32(3), *newRS.Spec.Replicas)
	assert.Equal(t, "2", newRS.ResourceVersion)
}

// TestConversionEventHandlerOnDelete covers all four OnDelete code paths:
//  1. a regular *v1.ReplicationController object,
//  2. a tombstone wrapping an RC (handler receives a tombstone wrapping the RS),
//  3. a tombstone wrapping a non-RC object (event dropped, handler not invoked),
//  4. a non-RC, non-tombstone object (event dropped, handler not invoked).
func TestConversionEventHandlerOnDelete(t *testing.T) {
	t.Run("regular RC object", func(t *testing.T) {
		rc := newTestRC("rc-del", "ns", 2, map[string]string{"app": "del"})
		fakeHandler := &fakeResourceEventHandler{}
		convHandler := conversionEventHandler{handler: fakeHandler}

		convHandler.OnDelete(rc.DeepCopy())

		_, _, deletes := fakeHandler.snapshot()
		require.Len(t, deletes, 1, "expected exactly one OnDelete call")
		rs, ok := deletes[0].(*apps.ReplicaSet)
		require.True(t, ok, "OnDelete should receive *apps.ReplicaSet; got %T", deletes[0])
		assert.Equal(t, "rc-del", rs.Name)
	})

	t.Run("tombstone wrapping RC", func(t *testing.T) {
		rc := newTestRC("rc-tomb", "ns", 4, map[string]string{"app": "tomb"})
		tombstone := cache.DeletedFinalStateUnknown{
			Key: "ns/rc-tomb",
			Obj: rc.DeepCopy(),
		}
		fakeHandler := &fakeResourceEventHandler{}
		convHandler := conversionEventHandler{handler: fakeHandler}

		convHandler.OnDelete(tombstone)

		_, _, deletes := fakeHandler.snapshot()
		require.Len(t, deletes, 1, "expected exactly one OnDelete call")
		// The downstream handler receives a tombstone wrapping the converted RS.
		outerTombstone, ok := deletes[0].(cache.DeletedFinalStateUnknown)
		require.True(t, ok, "OnDelete should receive cache.DeletedFinalStateUnknown; got %T", deletes[0])
		assert.Equal(t, "ns/rc-tomb", outerTombstone.Key)
		rs, ok := outerTombstone.Obj.(*apps.ReplicaSet)
		require.True(t, ok, "Tombstone.Obj should be *apps.ReplicaSet; got %T", outerTombstone.Obj)
		assert.Equal(t, "rc-tomb", rs.Name)
		require.NotNil(t, rs.Spec.Replicas)
		assert.Equal(t, int32(4), *rs.Spec.Replicas)
	})

	t.Run("tombstone wrapping non-RC drops event", func(t *testing.T) {
		// A tombstone wrapping a non-RC object (e.g., a Pod): the production code
		// calls utilruntime.HandleError and returns without invoking the
		// downstream handler.
		tombstone := cache.DeletedFinalStateUnknown{
			Key: "ns/some-pod",
			Obj: &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "some-pod", Namespace: "ns"}},
		}
		fakeHandler := &fakeResourceEventHandler{}
		convHandler := conversionEventHandler{handler: fakeHandler}

		convHandler.OnDelete(tombstone)

		_, _, deletes := fakeHandler.snapshot()
		assert.Empty(t, deletes, "downstream handler should NOT be invoked for non-RC tombstone")
	})

	t.Run("non-RC, non-tombstone drops event", func(t *testing.T) {
		// A non-RC, non-tombstone object: the production code calls
		// utilruntime.HandleError and returns without invoking the downstream
		// handler.
		fakeHandler := &fakeResourceEventHandler{}
		convHandler := conversionEventHandler{handler: fakeHandler}

		convHandler.OnDelete(&v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "stray-pod", Namespace: "ns"}})

		_, _, deletes := fakeHandler.snapshot()
		assert.Empty(t, deletes, "downstream handler should NOT be invoked for non-RC, non-tombstone object")
	})
}

// TestClientsetAdapter verifies that clientsetAdapter exposes the conversion
// app clients, and that the resulting ReplicaSets client is a conversionClient.
func TestClientsetAdapter(t *testing.T) {
	// newAdapter builds a fresh clientsetAdapter (with its own fake clientset) so
	// each subtest owns isolated state and is independently runnable.
	newAdapter := func() clientsetAdapter {
		return clientsetAdapter{Interface: fake.NewSimpleClientset()}
	}

	t.Run("AppsV1 returns conversionAppsV1Client", func(t *testing.T) {
		adapter := newAdapter()
		got := adapter.AppsV1()
		require.NotNil(t, got)
		_, ok := got.(conversionAppsV1Client)
		assert.True(t, ok, "AppsV1() should return conversionAppsV1Client; got %T", got)
	})

	t.Run("Apps returns conversionAppsV1Client", func(t *testing.T) {
		adapter := newAdapter()
		got := adapter.Apps()
		require.NotNil(t, got)
		_, ok := got.(conversionAppsV1Client)
		assert.True(t, ok, "Apps() should return conversionAppsV1Client; got %T", got)
	})

	t.Run("ReplicaSets returns conversionClient", func(t *testing.T) {
		adapter := newAdapter()
		got := adapter.AppsV1().ReplicaSets("ns")
		require.NotNil(t, got)
		_, ok := got.(conversionClient)
		assert.True(t, ok, "ReplicaSets() should return conversionClient; got %T", got)
	})
}

// TestConversionClientCreate verifies that Create converts the RS to an RC,
// issues a create against the underlying replicationcontrollers resource, and
// converts the result back to an RS. The AlreadyExists error path is injected
// via a reactor and asserted to propagate.
func TestConversionClientCreate(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		_, ctx := ktesting.NewTestContext(t)
		client := fake.NewSimpleClientset()
		convClient := clientsetAdapter{Interface: client}.AppsV1().ReplicaSets("ns")

		rs := newTestRS("rs-create", "ns", 2, map[string]string{"app": "create"})

		result, err := convClient.Create(ctx, rs, metav1.CreateOptions{})
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "rs-create", result.Name)
		require.NotNil(t, result.Spec.Replicas)
		assert.Equal(t, int32(2), *result.Spec.Replicas)

		// The underlying operation must target replicationcontrollers.
		var foundCreate bool
		for _, action := range client.Actions() {
			if action.GetVerb() == "create" && action.GetResource().Resource == "replicationcontrollers" {
				foundCreate = true
				break
			}
		}
		assert.True(t, foundCreate, "expected a create action on replicationcontrollers")
	})

	t.Run("propagates AlreadyExists error", func(t *testing.T) {
		_, ctx := ktesting.NewTestContext(t)
		client := fake.NewSimpleClientset()
		client.PrependReactor("create", "replicationcontrollers", func(action core.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewAlreadyExists(schema.GroupResource{Resource: "replicationcontrollers"}, "rs-create")
		})
		convClient := clientsetAdapter{Interface: client}.AppsV1().ReplicaSets("ns")

		rs := newTestRS("rs-create", "ns", 2, map[string]string{"app": "create"})

		result, err := convClient.Create(ctx, rs, metav1.CreateOptions{})
		assert.Nil(t, result)
		require.Error(t, err)
		assert.True(t, apierrors.IsAlreadyExists(err), "expected AlreadyExists; got %v", err)
	})
}

// TestConversionClientUpdate verifies the Update happy path and that a Conflict
// error from the underlying client propagates.
func TestConversionClientUpdate(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		_, ctx := ktesting.NewTestContext(t)
		existing := newTestRC("rs-update", "ns", 2, map[string]string{"app": "upd"})
		client := fake.NewSimpleClientset(existing)
		convClient := clientsetAdapter{Interface: client}.AppsV1().ReplicaSets("ns")

		updated := newTestRS("rs-update", "ns", 5, map[string]string{"app": "upd"})
		updated.ResourceVersion = "1"

		result, err := convClient.Update(ctx, updated, metav1.UpdateOptions{})
		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, result.Spec.Replicas)
		assert.Equal(t, int32(5), *result.Spec.Replicas)
	})

	t.Run("propagates Conflict error", func(t *testing.T) {
		_, ctx := ktesting.NewTestContext(t)
		client := fake.NewSimpleClientset()
		client.PrependReactor("update", "replicationcontrollers", func(action core.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewConflict(
				schema.GroupResource{Resource: "replicationcontrollers"},
				"rs-update",
				assert.AnError,
			)
		})
		convClient := clientsetAdapter{Interface: client}.AppsV1().ReplicaSets("ns")

		updated := newTestRS("rs-update", "ns", 5, map[string]string{"app": "upd"})
		result, err := convClient.Update(ctx, updated, metav1.UpdateOptions{})
		assert.Nil(t, result)
		require.Error(t, err)
		assert.True(t, apierrors.IsConflict(err), "expected Conflict; got %v", err)
	})
}

// TestConversionClientUpdateStatus verifies that UpdateStatus targets the status
// subresource of the underlying replicationcontrollers resource.
func TestConversionClientUpdateStatus(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		_, ctx := ktesting.NewTestContext(t)
		existing := newTestRC("rs-status", "ns", 2, map[string]string{"app": "stat"})
		client := fake.NewSimpleClientset(existing)
		convClient := clientsetAdapter{Interface: client}.AppsV1().ReplicaSets("ns")

		rs := newTestRS("rs-status", "ns", 2, map[string]string{"app": "stat"})
		rs.ResourceVersion = "1"
		rs.Status.Replicas = 5
		rs.Status.ReadyReplicas = 4

		result, err := convClient.UpdateStatus(ctx, rs, metav1.UpdateOptions{})
		require.NoError(t, err)
		require.NotNil(t, result)

		var foundStatusUpdate bool
		for _, action := range client.Actions() {
			if action.GetVerb() == "update" && action.GetSubresource() == "status" {
				foundStatusUpdate = true
				break
			}
		}
		assert.True(t, foundStatusUpdate, "expected an update action on the status subresource")
	})
}

// TestConversionClientGet verifies the Get happy path and the NotFound path.
func TestConversionClientGet(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		_, ctx := ktesting.NewTestContext(t)
		existing := newTestRC("rs-get", "ns", 3, map[string]string{"app": "get"})
		client := fake.NewSimpleClientset(existing)
		convClient := clientsetAdapter{Interface: client}.AppsV1().ReplicaSets("ns")

		result, err := convClient.Get(ctx, "rs-get", metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "rs-get", result.Name)
		require.NotNil(t, result.Spec.Replicas)
		assert.Equal(t, int32(3), *result.Spec.Replicas)
	})

	t.Run("NotFound", func(t *testing.T) {
		_, ctx := ktesting.NewTestContext(t)
		client := fake.NewSimpleClientset()
		convClient := clientsetAdapter{Interface: client}.AppsV1().ReplicaSets("ns")

		result, err := convClient.Get(ctx, "missing", metav1.GetOptions{})
		assert.Nil(t, result)
		require.Error(t, err)
		assert.True(t, apierrors.IsNotFound(err), "expected NotFound; got %v", err)
	})
}

// TestConversionClientList verifies that List converts the underlying RC list
// into an RS list.
func TestConversionClientList(t *testing.T) {
	_, ctx := ktesting.NewTestContext(t)
	rc1 := newTestRC("rs-list-1", "ns", 1, map[string]string{"app": "a"})
	rc2 := newTestRC("rs-list-2", "ns", 2, map[string]string{"app": "b"})
	client := fake.NewSimpleClientset(rc1, rc2)
	convClient := clientsetAdapter{Interface: client}.AppsV1().ReplicaSets("ns")

	result, err := convClient.List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Items, 2)
	names := []string{result.Items[0].Name, result.Items[1].Name}
	assert.Contains(t, names, "rs-list-1")
	assert.Contains(t, names, "rs-list-2")
}

// TestConversionClientNotImplemented verifies that every method the
// conversionClient deliberately does not implement returns its exact,
// production-defined error string. These methods are intentionally unused by
// the ReplicaSetController (which wraps the shared informer and does not call
// Watch/Patch/Apply/Scale through this client).
func TestConversionClientNotImplemented(t *testing.T) {
	// The call closures receive a freshly-constructed ctx and conversionClient
	// (built per subtest in the loop below) so no fake-clientset or conversion
	// client state is shared across cases.
	tests := []struct {
		name    string
		call    func(ctx context.Context, c appsv1client.ReplicaSetInterface) (interface{}, error)
		wantErr string
	}{
		{
			name: "Watch",
			call: func(ctx context.Context, c appsv1client.ReplicaSetInterface) (interface{}, error) {
				return c.Watch(ctx, metav1.ListOptions{})
			},
			wantErr: "Watch() is not implemented for conversionClient",
		},
		{
			name: "Patch",
			call: func(ctx context.Context, c appsv1client.ReplicaSetInterface) (interface{}, error) {
				return c.Patch(ctx, "name", types.JSONPatchType, []byte("[]"), metav1.PatchOptions{})
			},
			wantErr: "Patch() is not implemented for conversionClient",
		},
		{
			name: "Apply",
			call: func(ctx context.Context, c appsv1client.ReplicaSetInterface) (interface{}, error) {
				cfg := appsv1apply.ReplicaSet("rs", "ns")
				return c.Apply(ctx, cfg, metav1.ApplyOptions{})
			},
			wantErr: "Apply() is not implemented for conversionClient",
		},
		{
			name: "ApplyStatus",
			call: func(ctx context.Context, c appsv1client.ReplicaSetInterface) (interface{}, error) {
				cfg := appsv1apply.ReplicaSet("rs", "ns")
				return c.ApplyStatus(ctx, cfg, metav1.ApplyOptions{})
			},
			wantErr: "ApplyStatus() is not implemented for conversionClient",
		},
		{
			name: "GetScale",
			call: func(ctx context.Context, c appsv1client.ReplicaSetInterface) (interface{}, error) {
				return c.GetScale(ctx, "rs", metav1.GetOptions{})
			},
			wantErr: "GetScale() is not implemented for conversionClient",
		},
		{
			name: "UpdateScale",
			call: func(ctx context.Context, c appsv1client.ReplicaSetInterface) (interface{}, error) {
				return c.UpdateScale(ctx, "rs", &autoscalingv1.Scale{}, metav1.UpdateOptions{})
			},
			wantErr: "UpdateScale() is not implemented for conversionClient",
		},
		{
			name: "ApplyScale",
			call: func(ctx context.Context, c appsv1client.ReplicaSetInterface) (interface{}, error) {
				// autoscaling/v1 ScaleApplyConfiguration is constructed with no
				// arguments (unlike apps/v1 ReplicaSet, which takes name+namespace).
				cfg := appsv1autoscaling.Scale()
				return c.ApplyScale(ctx, "rs", cfg, metav1.ApplyOptions{})
			},
			wantErr: "ApplyScale() is not implemented for conversionClient",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Fresh fake clientset and conversion client per subtest: no shared
			// mutable state, so each case is independently runnable.
			_, ctx := ktesting.NewTestContext(t)
			convClient := clientsetAdapter{Interface: fake.NewSimpleClientset()}.AppsV1().ReplicaSets("ns")
			result, err := tc.call(ctx, convClient)
			require.Error(t, err, "%s should return an error", tc.name)
			assert.Equal(t, tc.wantErr, err.Error(), "%s error message mismatch", tc.name)
			_ = result
		})
	}
}

// TestPodControlAdapterCreatePods verifies that podControlAdapter.CreatePods
// converts the RS argument to an RC and delegates to the wrapped
// PodControlInterface with the original template and controller reference.
func TestPodControlAdapterCreatePods(t *testing.T) {
	_, ctx := ktesting.NewTestContext(t)
	fakePC := &controller.FakePodControl{}
	adapter := podControlAdapter{PodControlInterface: fakePC}

	rs := newTestRS("rs-create-pod", "ns", 1, map[string]string{"app": "create-pod"})
	template := &v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "create-pod"}},
		Spec: v1.PodSpec{
			Containers:    []v1.Container{{Name: "c", Image: "nginx"}},
			RestartPolicy: v1.RestartPolicyAlways,
			DNSPolicy:     v1.DNSClusterFirst,
		},
	}
	trueVal := true
	ownerRef := &metav1.OwnerReference{
		APIVersion:         "v1",
		Kind:               "ReplicationController",
		Name:               rs.Name,
		UID:                rs.UID,
		Controller:         &trueVal,
		BlockOwnerDeletion: &trueVal,
	}

	err := adapter.CreatePods(ctx, "ns", template, rs, ownerRef)
	require.NoError(t, err)
	require.Len(t, fakePC.Templates, 1, "expected one CreatePods call")
	require.Len(t, fakePC.ControllerRefs, 1, "expected one controllerRef recorded")
	assert.Equal(t, rs.Name, fakePC.ControllerRefs[0].Name)
}

// TestPodControlAdapterDeletePod verifies that podControlAdapter.DeletePod
// converts the RS argument to an RC and delegates to the wrapped
// PodControlInterface with the pod identifier.
func TestPodControlAdapterDeletePod(t *testing.T) {
	_, ctx := ktesting.NewTestContext(t)
	fakePC := &controller.FakePodControl{}
	adapter := podControlAdapter{PodControlInterface: fakePC}

	rs := newTestRS("rs-delete-pod", "ns", 1, map[string]string{"app": "delete-pod"})

	err := adapter.DeletePod(ctx, "ns", "pod-x", rs)
	require.NoError(t, err)
	require.Len(t, fakePC.DeletePodName, 1)
	assert.Equal(t, "pod-x", fakePC.DeletePodName[0])
}

// TestBurstReplicasConstant is a lightweight sanity check that the package-local
// BurstReplicas constant re-exports the replicaset package's value.
func TestBurstReplicasConstant(t *testing.T) {
	assert.Equal(t, replicaset.BurstReplicas, BurstReplicas)
}
