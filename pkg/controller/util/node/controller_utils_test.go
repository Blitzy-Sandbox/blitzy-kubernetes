/*
Copyright The Kubernetes Authors.

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

package node

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/fake"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	core "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2/ktesting"

	_ "k8s.io/kubernetes/pkg/apis/core/install"
	"k8s.io/kubernetes/pkg/controller/testutil"
	nodepkg "k8s.io/kubernetes/pkg/util/node"
)

// drainEvents returns all messages currently buffered on the FakeRecorder channel
// without blocking. It MUST be called only after the unit under test has returned,
// so all synchronously-emitted events are already in the buffer.
func drainEvents(t *testing.T, ch <-chan string) []string {
	t.Helper()
	var got []string
	for {
		select {
		case e := <-ch:
			got = append(got, e)
		default:
			return got
		}
	}
}

// countPodActions returns the count of fake-clientset actions matching the
// supplied verb on the "pods" resource. Filter by subresource by passing
// a non-empty subresource (e.g., "status"); pass "" to match any subresource.
func countPodActions(actions []core.Action, verb, subresource string) int {
	n := 0
	for _, a := range actions {
		if a.GetVerb() != verb {
			continue
		}
		if a.GetResource().Resource != "pods" {
			continue
		}
		if subresource != "" && a.GetSubresource() != subresource {
			continue
		}
		n++
	}
	return n
}

// countNodeActions returns the count of fake-clientset actions matching the
// supplied verb on the "nodes" resource.
func countNodeActions(actions []core.Action, verb string) int {
	n := 0
	for _, a := range actions {
		if a.GetVerb() == verb && a.GetResource().Resource == "nodes" {
			n++
		}
	}
	return n
}

// newDaemonSetLister builds an in-memory DaemonSetLister backed by a fresh
// cache.Indexer. Pass any number of DaemonSets to pre-populate the lister.
// The lister's GetPodDaemonSets returns (nonEmpty, nil) when at least one
// DaemonSet's Selector matches a pod's labels in the same namespace.
func newDaemonSetLister(t *testing.T, dss ...*appsv1.DaemonSet) appsv1listers.DaemonSetLister {
	t.Helper()
	idx := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	for _, ds := range dss {
		require.NoError(t, idx.Add(ds))
	}
	return appsv1listers.NewDaemonSetLister(idx)
}

// podOnNode returns a fresh Pod scheduled to host with the supplied labels.
func podOnNode(name, host string, labels map[string]string) *v1.Pod {
	p := testutil.NewPod(name, host)
	if labels != nil {
		p.Labels = labels
	}
	return p
}

func TestDeletePods(t *testing.T) {
	const (
		nodeName = "n1"
		nodeUID  = "node-uid-123"
	)

	// gpPod has a deletion grace period set; production returns remaining=true for
	// it without issuing a Delete and without emitting a per-pod eviction event.
	newGracePeriodPod := func() *v1.Pod {
		p := testutil.NewPod("p1", nodeName)
		gp := int64(30)
		p.DeletionGracePeriodSeconds = &gp
		return p
	}

	// matchingDaemonSet selects pods carrying the {"app":"ds"} label in the
	// "default" namespace, exercising the DaemonSet-skip branch of DeletePods.
	matchingDaemonSet := func() *appsv1.DaemonSet {
		return &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{Name: "myds", Namespace: metav1.NamespaceDefault},
			Spec:       appsv1.DaemonSetSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "ds"}}},
		}
	}

	// newMirrorPod returns a pod scheduled to nodeName that carries the kubelet
	// mirror-pod annotation (v1.MirrorPodAnnotationKey == "kubernetes.io/config.mirror").
	//
	// AAP/production mismatch (documented intentionally, not silently omitted):
	// the AAP test blueprint listed a "pod with mirror-pod annotation skipped"
	// case for DeletePods, but the production DeletePods (controller_utils.go)
	// has NO mirror-pod branch. It only skips a pod when (a) the pod's
	// Spec.NodeName differs from the target node, (b) DeletionGracePeriodSeconds
	// is non-nil, or (c) the pod is owned by a DaemonSet. A mirror pod matches
	// none of those, so it is treated like any other pod and IS deleted. The row
	// below asserts that actual production behavior and records the mismatch so a
	// future decision to special-case mirror pods (a production change, out of
	// scope here) can be made deliberately rather than by accident.
	newMirrorPod := func() *v1.Pod {
		p := testutil.NewPod("p1", nodeName)
		p.Annotations = map[string]string{v1.MirrorPodAnnotationKey: ""}
		return p
	}

	tests := []struct {
		name              string
		pods              []*v1.Pod
		dss               []*appsv1.DaemonSet
		reactor           func(*fake.Clientset)
		wantRemaining     bool
		wantErr           bool
		wantErrIsConflict bool
		wantDeleteActions int
		wantUpdateStatus  int
		wantEvents        int
	}{
		{
			name:              "empty_pod_slice",
			pods:              nil,
			wantRemaining:     false,
			wantErr:           false,
			wantDeleteActions: 0,
			wantUpdateStatus:  0,
			wantEvents:        0,
		},
		{
			name:              "single_pod_successful_delete",
			pods:              []*v1.Pod{testutil.NewPod("p1", nodeName)},
			wantRemaining:     true,
			wantErr:           false,
			wantDeleteActions: 1,
			wantUpdateStatus:  1,
			// DeletingAllPods (node) + NodeControllerEviction (pod).
			wantEvents: 2,
		},
		{
			name:              "pod_with_different_NodeName_is_skipped",
			pods:              []*v1.Pod{testutil.NewPod("p1", "other")},
			wantRemaining:     false,
			wantErr:           false,
			wantDeleteActions: 0,
			wantUpdateStatus:  0,
			// Only the DeletingAllPods event because len(pods) > 0.
			wantEvents: 1,
		},
		{
			name:              "pod_with_DeletionGracePeriodSeconds_is_kept_remaining",
			pods:              []*v1.Pod{newGracePeriodPod()},
			wantRemaining:     true,
			wantErr:           false,
			wantDeleteActions: 0,
			// SetPodTerminationReason still runs before the grace-period short-circuit.
			wantUpdateStatus: 1,
			wantEvents:       1,
		},
		{
			name:              "pod_owned_by_DaemonSet_is_skipped",
			pods:              []*v1.Pod{podOnNode("p1", nodeName, map[string]string{"app": "ds"})},
			dss:               []*appsv1.DaemonSet{matchingDaemonSet()},
			wantRemaining:     false,
			wantErr:           false,
			wantDeleteActions: 0,
			// SetPodTerminationReason runs before the DaemonSet check.
			wantUpdateStatus: 1,
			wantEvents:       1,
		},
		{
			// Mirror pods are NOT skipped by DeletePods: production has no
			// mirror-pod branch (see newMirrorPod above for the full rationale),
			// so the pod is deleted exactly like single_pod_successful_delete.
			name:              "mirror_pod_is_NOT_skipped_no_production_mirror_branch",
			pods:              []*v1.Pod{newMirrorPod()},
			wantRemaining:     true,
			wantErr:           false,
			wantDeleteActions: 1,
			wantUpdateStatus:  1,
			// DeletingAllPods (node) + NodeControllerEviction (pod).
			wantEvents: 2,
		},
		{
			name: "API_NotFound_on_Delete_is_treated_as_success",
			pods: []*v1.Pod{testutil.NewPod("p1", nodeName)},
			reactor: func(c *fake.Clientset) {
				c.PrependReactor("delete", "pods", func(action core.Action) (bool, runtime.Object, error) {
					return true, nil, apierrors.NewNotFound(schema.GroupResource{Resource: "pods"}, action.(core.DeleteAction).GetName())
				})
			},
			// Production continues past NotFound and never sets remaining=true.
			wantRemaining:     false,
			wantErr:           false,
			wantDeleteActions: 1,
			wantUpdateStatus:  1,
			wantEvents:        2,
		},
		{
			name: "API_Conflict_on_Delete_returns_error_immediately",
			pods: []*v1.Pod{testutil.NewPod("p1", nodeName)},
			reactor: func(c *fake.Clientset) {
				c.PrependReactor("delete", "pods", func(action core.Action) (bool, runtime.Object, error) {
					return true, nil, apierrors.NewConflict(schema.GroupResource{Resource: "pods"}, action.(core.DeleteAction).GetName(), errors.New("rv mismatch"))
				})
			},
			wantRemaining:     false,
			wantErr:           true,
			wantErrIsConflict: true,
			wantDeleteActions: 1,
			wantUpdateStatus:  1,
			wantEvents:        2,
		},
		{
			name: "Conflict_on_UpdateStatus_is_aggregated_and_pod_skipped",
			pods: []*v1.Pod{testutil.NewPod("p1", nodeName)},
			reactor: func(c *fake.Clientset) {
				c.PrependReactor("update", "pods", func(action core.Action) (bool, runtime.Object, error) {
					if action.GetSubresource() == "status" {
						return true, nil, apierrors.NewConflict(schema.GroupResource{Resource: "pods"}, "p1", errors.New("rv mismatch"))
					}
					return false, nil, nil
				})
			},
			// Conflict from SetPodTerminationReason causes a continue, so the pod is
			// never deleted and no per-pod eviction event is emitted. The aggregate
			// error is not itself an APIStatus, so IsConflict on it is false.
			wantRemaining:     false,
			wantErr:           true,
			wantErrIsConflict: false,
			wantDeleteActions: 0,
			wantUpdateStatus:  1,
			wantEvents:        1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)

			// Pre-seed pods in the clientset so UpdateStatus/Delete have a real
			// object to act upon. Each t.Run builds fresh fakes (no shared state).
			var initialObjs []runtime.Object
			for _, p := range tc.pods {
				initialObjs = append(initialObjs, p)
			}

			client := fake.NewSimpleClientset(initialObjs...)
			if tc.reactor != nil {
				tc.reactor(client)
			}
			recorder := record.NewFakeRecorder(16)
			dsLister := newDaemonSetLister(t, tc.dss...)

			remaining, err := DeletePods(ctx, client, tc.pods, recorder, nodeName, nodeUID, dsLister)

			if tc.wantErr {
				require.Error(t, err)
				if tc.wantErrIsConflict {
					assert.True(t, apierrors.IsConflict(err), "expected a Conflict error, got %v", err)
				}
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.wantRemaining, remaining, "remaining return value")

			actions := client.Actions()
			assert.Equal(t, tc.wantDeleteActions, countPodActions(actions, "delete", ""), "delete pod actions")
			assert.Equal(t, tc.wantUpdateStatus, countPodActions(actions, "update", "status"), "update pod/status actions")

			events := drainEvents(t, recorder.Events)
			assert.Len(t, events, tc.wantEvents, "emitted events: %v", events)
		})
	}
}

func TestSetPodTerminationReason(t *testing.T) {
	const nodeName = "n1"

	tests := []struct {
		name             string
		podFn            func() *v1.Pod
		seed             bool
		reactor          func(*fake.Clientset)
		wantNilResult    bool
		wantSamePointer  bool
		wantErr          bool
		wantErrConflict  bool
		wantUpdateStatus int
	}{
		{
			name:             "fresh_pod_gets_reason_set",
			podFn:            func() *v1.Pod { return testutil.NewPod("p1", nodeName) },
			seed:             true,
			wantUpdateStatus: 1,
		},
		{
			name: "already_NodeLost_returns_immediately",
			podFn: func() *v1.Pod {
				p := testutil.NewPod("p1", nodeName)
				p.Status.Reason = nodepkg.NodeUnreachablePodReason
				return p
			},
			// No clientset interaction expected; the function short-circuits.
			seed:             false,
			wantSamePointer:  true,
			wantUpdateStatus: 0,
		},
		{
			name:  "UpdateStatus_Conflict_propagated",
			podFn: func() *v1.Pod { return testutil.NewPod("p1", nodeName) },
			seed:  true,
			reactor: func(c *fake.Clientset) {
				c.PrependReactor("update", "pods", func(action core.Action) (bool, runtime.Object, error) {
					if action.GetSubresource() == "status" {
						return true, nil, apierrors.NewConflict(schema.GroupResource{Resource: "pods"}, "p1", errors.New("rv mismatch"))
					}
					return false, nil, nil
				})
			},
			wantNilResult:    true,
			wantErr:          true,
			wantErrConflict:  true,
			wantUpdateStatus: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)

			pod := tc.podFn()
			var initialObjs []runtime.Object
			if tc.seed {
				initialObjs = append(initialObjs, pod)
			}
			client := fake.NewSimpleClientset(initialObjs...)
			if tc.reactor != nil {
				tc.reactor(client)
			}

			got, err := SetPodTerminationReason(ctx, client, pod, nodeName)

			if tc.wantErr {
				require.Error(t, err)
				if tc.wantErrConflict {
					assert.True(t, apierrors.IsConflict(err), "expected a Conflict error, got %v", err)
				}
			} else {
				require.NoError(t, err)
			}

			if tc.wantNilResult {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
			}

			if tc.wantSamePointer {
				// Idempotent short-circuit must return the exact input pointer.
				assert.Same(t, pod, got)
			}

			if !tc.wantErr && !tc.wantSamePointer {
				// The fresh-pod path stamps the unreachable reason and message.
				assert.Equal(t, nodepkg.NodeUnreachablePodReason, got.Status.Reason)
				wantMsg := fmt.Sprintf(nodepkg.NodeUnreachablePodMessage, nodeName, "p1")
				assert.Equal(t, wantMsg, got.Status.Message)
			}

			assert.Equal(t, tc.wantUpdateStatus, countPodActions(client.Actions(), "update", "status"), "update pod/status actions")
		})
	}
}

func TestMarkPodsNotReady(t *testing.T) {
	const nodeName = "n1"

	tests := []struct {
		name             string
		podFn            func() *v1.Pod
		reactor          func(*fake.Clientset)
		wantErr          bool
		wantUpdateStatus int
		wantEvents       int
		wantEventSubstr  string
	}{
		{
			name:             "transitions_PodReady_True_to_False",
			podFn:            func() *v1.Pod { return testutil.NewPod("p1", nodeName) },
			wantUpdateStatus: 1,
			wantEvents:       1,
			wantEventSubstr:  "Warning NodeNotReady Node is not ready",
		},
		{
			name:             "different_NodeName_is_skipped",
			podFn:            func() *v1.Pod { return testutil.NewPod("p1", "other") },
			wantUpdateStatus: 0,
			wantEvents:       0,
		},
		{
			name: "already_False_short_circuits_inner_loop",
			podFn: func() *v1.Pod {
				p := testutil.NewPod("p1", nodeName)
				p.Status.Conditions[0].Status = v1.ConditionFalse
				return p
			},
			wantUpdateStatus: 0,
			wantEvents:       0,
		},
		{
			name: "no_PodReady_condition",
			podFn: func() *v1.Pod {
				p := testutil.NewPod("p1", nodeName)
				p.Status.Conditions = nil
				return p
			},
			wantUpdateStatus: 0,
			wantEvents:       0,
		},
		{
			name: "skips_non_PodReady_condition_then_transitions",
			podFn: func() *v1.Pod {
				// A non-Ready condition precedes PodReady, exercising the
				// cond.Type != v1.PodReady continue branch before the transition.
				p := testutil.NewPod("p1", nodeName)
				p.Status.Conditions = append(
					[]v1.PodCondition{{Type: v1.PodScheduled, Status: v1.ConditionTrue}},
					p.Status.Conditions...,
				)
				return p
			},
			wantUpdateStatus: 1,
			wantEvents:       1,
			wantEventSubstr:  "Warning NodeNotReady Node is not ready",
		},
		{
			name:  "NotFound_silently_swallowed",
			podFn: func() *v1.Pod { return testutil.NewPod("p1", nodeName) },
			reactor: func(c *fake.Clientset) {
				c.PrependReactor("update", "pods", func(action core.Action) (bool, runtime.Object, error) {
					if action.GetSubresource() == "status" {
						return true, nil, apierrors.NewNotFound(schema.GroupResource{Resource: "pods"}, "p1")
					}
					return false, nil, nil
				})
			},
			wantUpdateStatus: 1,
			// NotFound triggers a continue that skips event emission.
			wantEvents: 0,
		},
		{
			name:  "Conflict_aggregated_event_still_emitted",
			podFn: func() *v1.Pod { return testutil.NewPod("p1", nodeName) },
			reactor: func(c *fake.Clientset) {
				c.PrependReactor("update", "pods", func(action core.Action) (bool, runtime.Object, error) {
					if action.GetSubresource() == "status" {
						return true, nil, apierrors.NewConflict(schema.GroupResource{Resource: "pods"}, "p1", errors.New("rv mismatch"))
					}
					return false, nil, nil
				})
			},
			// A non-NotFound error is aggregated but the event still fires.
			wantErr:          true,
			wantUpdateStatus: 1,
			wantEvents:       1,
			wantEventSubstr:  "Warning NodeNotReady Node is not ready",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)

			pod := tc.podFn()
			client := fake.NewSimpleClientset(pod)
			if tc.reactor != nil {
				tc.reactor(client)
			}
			recorder := record.NewFakeRecorder(16)

			err := MarkPodsNotReady(ctx, client, recorder, []*v1.Pod{pod}, nodeName)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tc.wantUpdateStatus, countPodActions(client.Actions(), "update", "status"), "update pod/status actions")

			events := drainEvents(t, recorder.Events)
			require.Len(t, events, tc.wantEvents, "emitted events: %v", events)
			if tc.wantEventSubstr != "" {
				assert.Contains(t, events[0], tc.wantEventSubstr)
			}
		})
	}
}

func TestRecordNodeEvent(t *testing.T) {
	_, ctx := ktesting.NewTestContext(t)
	recorder := record.NewFakeRecorder(4)

	RecordNodeEvent(ctx, recorder, "n1", "uid-xyz", v1.EventTypeNormal, "TestReason", "TestEvent")

	events := drainEvents(t, recorder.Events)
	require.Len(t, events, 1)
	want := fmt.Sprintf("%s %s Node %s event: %s", v1.EventTypeNormal, "TestReason", "n1", "TestEvent")
	assert.Equal(t, want, events[0])
}

func TestRecordNodeStatusChange(t *testing.T) {
	logger, _ := ktesting.NewTestContext(t)
	recorder := record.NewFakeRecorder(4)

	node := testutil.NewNode("n1")
	node.UID = "uid-xyz"

	RecordNodeStatusChange(logger, recorder, node, "Ready")

	events := drainEvents(t, recorder.Events)
	require.Len(t, events, 1)
	want := fmt.Sprintf("%s %s Node %s status is now: %s", v1.EventTypeNormal, "Ready", "n1", "Ready")
	assert.Equal(t, want, events[0])
}

func TestSwapNodeControllerTaint(t *testing.T) {
	const nodeName = "n1"

	tests := []struct {
		name           string
		nodeFn         func() *v1.Node
		taintsToAdd    []*v1.Taint
		taintsToRemove []*v1.Taint
		reactor        func(*fake.Clientset)
		want           bool
		wantGetAtLeast int
		wantPatch      int
	}{
		{
			name:           "add_taint_when_absent",
			nodeFn:         func() *v1.Node { return testutil.NewNode(nodeName) },
			taintsToAdd:    []*v1.Taint{{Key: "x", Value: "y", Effect: v1.TaintEffectNoSchedule}},
			taintsToRemove: nil,
			want:           true,
			wantGetAtLeast: 1,
			wantPatch:      1,
		},
		{
			name: "remove_existing_taint",
			nodeFn: func() *v1.Node {
				n := testutil.NewNode(nodeName)
				n.Spec.Taints = []v1.Taint{{Key: "x", Effect: v1.TaintEffectNoSchedule}}
				return n
			},
			taintsToAdd:    nil,
			taintsToRemove: []*v1.Taint{{Key: "x", Effect: v1.TaintEffectNoSchedule}},
			want:           true,
			wantGetAtLeast: 1,
			wantPatch:      1,
		},
		{
			name:           "noop_when_nothing_to_change",
			nodeFn:         func() *v1.Node { return testutil.NewNode(nodeName) },
			taintsToAdd:    nil,
			taintsToRemove: nil,
			// Both inner helpers short-circuit on empty taint lists.
			want:           true,
			wantGetAtLeast: 0,
			wantPatch:      0,
		},
		{
			name:        "AddOrUpdate_error_returns_false",
			nodeFn:      func() *v1.Node { return testutil.NewNode(nodeName) },
			taintsToAdd: []*v1.Taint{{Key: "x", Value: "y", Effect: v1.TaintEffectNoSchedule}},
			reactor: func(c *fake.Clientset) {
				// A NON-Conflict error is returned so RetryOnConflict does not retry
				// (a Conflict here would trigger real backoff sleeps).
				c.PrependReactor("get", "nodes", func(action core.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("network down")
				})
			},
			want:           false,
			wantGetAtLeast: 1,
			wantPatch:      0,
		},
		{
			name: "Remove_error_returns_false",
			nodeFn: func() *v1.Node {
				// The node already carries the taint so RemoveTaintOffNode does not
				// short-circuit and reaches the Get that the reactor fails.
				n := testutil.NewNode(nodeName)
				n.Spec.Taints = []v1.Taint{{Key: "x", Effect: v1.TaintEffectNoSchedule}}
				return n
			},
			taintsToAdd:    nil,
			taintsToRemove: []*v1.Taint{{Key: "x", Effect: v1.TaintEffectNoSchedule}},
			reactor: func(c *fake.Clientset) {
				c.PrependReactor("get", "nodes", func(action core.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("network down")
				})
			},
			want:           false,
			wantGetAtLeast: 1,
			wantPatch:      0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)

			node := tc.nodeFn()
			client := fake.NewSimpleClientset(node)
			if tc.reactor != nil {
				tc.reactor(client)
			}

			got := SwapNodeControllerTaint(ctx, client, tc.taintsToAdd, tc.taintsToRemove, node)
			assert.Equal(t, tc.want, got, "SwapNodeControllerTaint return value")

			actions := client.Actions()
			gets := countNodeActions(actions, "get")
			patches := countNodeActions(actions, "patch")
			assert.GreaterOrEqual(t, gets, tc.wantGetAtLeast, "get nodes actions")
			assert.Equal(t, tc.wantPatch, patches, "patch nodes actions")
		})
	}
}

func TestAddOrUpdateLabelsOnNode(t *testing.T) {
	const nodeName = "n1"

	tests := []struct {
		name           string
		labelsToUpdate map[string]string
		reactor        func(*fake.Clientset)
		want           bool
		wantGetAtLeast int
		wantPatch      int
	}{
		{
			name:           "applies_new_labels",
			labelsToUpdate: map[string]string{"foo": "bar"},
			want:           true,
			wantGetAtLeast: 1,
			wantPatch:      1,
		},
		{
			name:           "API_error_returns_false",
			labelsToUpdate: map[string]string{"foo": "bar"},
			reactor: func(c *fake.Clientset) {
				// NON-Conflict error to avoid RetryOnConflict backoff sleeps.
				c.PrependReactor("get", "nodes", func(action core.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("unavailable")
				})
			},
			want:           false,
			wantGetAtLeast: 1,
			wantPatch:      0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)

			node := testutil.NewNode(nodeName)
			client := fake.NewSimpleClientset(node)
			if tc.reactor != nil {
				tc.reactor(client)
			}

			got := AddOrUpdateLabelsOnNode(ctx, client, tc.labelsToUpdate, node)
			assert.Equal(t, tc.want, got, "AddOrUpdateLabelsOnNode return value")

			actions := client.Actions()
			assert.GreaterOrEqual(t, countNodeActions(actions, "get"), tc.wantGetAtLeast, "get nodes actions")
			assert.Equal(t, tc.wantPatch, countNodeActions(actions, "patch"), "patch nodes actions")
		})
	}
}

func TestCreateAddNodeHandler(t *testing.T) {
	t.Run("invokes_f_with_deep_copied_node", func(t *testing.T) {
		input := testutil.NewNode("n1")
		input.Labels = map[string]string{"orig": "value"}

		var captured *v1.Node
		handler := CreateAddNodeHandler(func(n *v1.Node) error {
			captured = n
			return nil
		})
		handler(input)

		require.NotNil(t, captured)
		assert.Equal(t, "n1", captured.Name)
		// Verify deep copy: mutating captured.Labels must not affect input.
		captured.Labels["mutated"] = "yes"
		_, isMutatedInInput := input.Labels["mutated"]
		assert.False(t, isMutatedInInput, "handler must pass a deep copy")
	})

	t.Run("f_error_invokes_utilruntime_HandleError", func(t *testing.T) {
		old := utilruntime.ErrorHandlers
		t.Cleanup(func() { utilruntime.ErrorHandlers = old })

		var captured error
		utilruntime.ErrorHandlers = []utilruntime.ErrorHandler{
			func(_ context.Context, err error, _ string, _ ...interface{}) {
				captured = err
			},
		}

		handler := CreateAddNodeHandler(func(*v1.Node) error { return errors.New("boom") })
		handler(testutil.NewNode("n1"))

		require.Error(t, captured)
		assert.Contains(t, captured.Error(), "boom")
	})
}

func TestCreateUpdateNodeHandler(t *testing.T) {
	t.Run("invokes_f_with_deep_copied_old_and_new", func(t *testing.T) {
		oldNode := testutil.NewNode("n1")
		oldNode.Labels = map[string]string{"v": "1"}
		newNode := testutil.NewNode("n1")
		newNode.Labels = map[string]string{"v": "2"}

		var capturedOld, capturedNew *v1.Node
		handler := CreateUpdateNodeHandler(func(o, n *v1.Node) error {
			capturedOld = o
			capturedNew = n
			return nil
		})
		handler(oldNode, newNode)

		require.NotNil(t, capturedOld)
		require.NotNil(t, capturedNew)
		assert.Equal(t, "1", capturedOld.Labels["v"])
		assert.Equal(t, "2", capturedNew.Labels["v"])

		// Verify deep copies for both arguments.
		capturedOld.Labels["mutated"] = "x"
		_, isMutated := oldNode.Labels["mutated"]
		assert.False(t, isMutated, "old node must be deep-copied")
		capturedNew.Labels["mutated"] = "x"
		_, isMutated = newNode.Labels["mutated"]
		assert.False(t, isMutated, "new node must be deep-copied")
	})

	t.Run("f_error_invokes_utilruntime_HandleError", func(t *testing.T) {
		old := utilruntime.ErrorHandlers
		t.Cleanup(func() { utilruntime.ErrorHandlers = old })

		var captured error
		utilruntime.ErrorHandlers = []utilruntime.ErrorHandler{
			func(_ context.Context, err error, _ string, _ ...interface{}) {
				captured = err
			},
		}

		handler := CreateUpdateNodeHandler(func(*v1.Node, *v1.Node) error { return errors.New("boom") })
		handler(testutil.NewNode("n1"), testutil.NewNode("n1"))

		require.Error(t, captured)
		assert.Contains(t, captured.Error(), "boom")
	})
}

func TestCreateDeleteNodeHandler(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		wantCall bool
		wantName string
	}{
		{
			name:     "regular_Node_invokes_f",
			input:    testutil.NewNode("n1"),
			wantCall: true,
			wantName: "n1",
		},
		{
			name: "tombstone_with_Node_invokes_f",
			input: cache.DeletedFinalStateUnknown{
				Key: "n1",
				Obj: testutil.NewNode("n1"),
			},
			wantCall: true,
			wantName: "n1",
		},
		{
			name: "tombstone_with_non_Node_logs_error_and_skips",
			input: cache.DeletedFinalStateUnknown{
				Key: "x",
				Obj: testutil.NewPod("p1", "h1"),
			},
			wantCall: false,
		},
		{
			name:     "unrelated_type_logs_error_and_skips",
			input:    "not-a-node",
			wantCall: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger, _ := ktesting.NewTestContext(t)

			called := false
			var captured *v1.Node
			handler := CreateDeleteNodeHandler(logger, func(n *v1.Node) error {
				called = true
				captured = n
				return nil
			})

			handler(tc.input)

			assert.Equal(t, tc.wantCall, called, "handler invocation flag")
			if tc.wantCall {
				require.NotNil(t, captured)
				assert.Equal(t, tc.wantName, captured.Name)
			} else {
				assert.Nil(t, captured)
			}
		})
	}

	t.Run("f_error_invokes_utilruntime_HandleError", func(t *testing.T) {
		logger, _ := ktesting.NewTestContext(t)

		old := utilruntime.ErrorHandlers
		t.Cleanup(func() { utilruntime.ErrorHandlers = old })

		var captured error
		utilruntime.ErrorHandlers = []utilruntime.ErrorHandler{
			func(_ context.Context, err error, _ string, _ ...interface{}) {
				captured = err
			},
		}

		handler := CreateDeleteNodeHandler(logger, func(*v1.Node) error { return errors.New("boom") })
		handler(testutil.NewNode("n1"))

		require.Error(t, captured)
		assert.Contains(t, captured.Error(), "boom")
	})
}

func TestGetNodeCondition(t *testing.T) {
	readyTrue := v1.NodeCondition{Type: v1.NodeReady, Status: v1.ConditionTrue}
	readyFalse := v1.NodeCondition{Type: v1.NodeReady, Status: v1.ConditionFalse}
	memPressure := v1.NodeCondition{Type: v1.NodeMemoryPressure, Status: v1.ConditionFalse}
	diskPressure := v1.NodeCondition{Type: v1.NodeDiskPressure, Status: v1.ConditionFalse}

	tests := []struct {
		name       string
		status     *v1.NodeStatus
		condType   v1.NodeConditionType
		wantIndex  int
		wantNil    bool
		wantStatus v1.ConditionStatus
		wantType   v1.NodeConditionType
	}{
		{
			name:       "found_at_index_0",
			status:     &v1.NodeStatus{Conditions: []v1.NodeCondition{readyTrue}},
			condType:   v1.NodeReady,
			wantIndex:  0,
			wantStatus: v1.ConditionTrue,
			wantType:   v1.NodeReady,
		},
		{
			name:       "found_at_later_index",
			status:     &v1.NodeStatus{Conditions: []v1.NodeCondition{memPressure, diskPressure, readyTrue}},
			condType:   v1.NodeReady,
			wantIndex:  2,
			wantStatus: v1.ConditionTrue,
			wantType:   v1.NodeReady,
		},
		{
			name:      "nil_status_returns_minus_one_nil",
			status:    nil,
			condType:  v1.NodeReady,
			wantIndex: -1,
			wantNil:   true,
		},
		{
			name:      "empty_conditions_returns_minus_one_nil",
			status:    &v1.NodeStatus{Conditions: nil},
			condType:  v1.NodeReady,
			wantIndex: -1,
			wantNil:   true,
		},
		{
			name:      "condition_not_present_returns_minus_one_nil",
			status:    &v1.NodeStatus{Conditions: []v1.NodeCondition{memPressure, diskPressure}},
			condType:  v1.NodeReady,
			wantIndex: -1,
			wantNil:   true,
		},
		{
			name:       "duplicate_types_returns_first_index",
			status:     &v1.NodeStatus{Conditions: []v1.NodeCondition{readyTrue, readyFalse}},
			condType:   v1.NodeReady,
			wantIndex:  0,
			wantStatus: v1.ConditionTrue,
			wantType:   v1.NodeReady,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			idx, got := GetNodeCondition(tc.status, tc.condType)
			assert.Equal(t, tc.wantIndex, idx)
			if tc.wantNil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, tc.wantStatus, got.Status)
				assert.Equal(t, tc.wantType, got.Type)
				// Verify the returned pointer aliases the slice element, not a copy.
				if idx >= 0 && tc.status != nil && idx < len(tc.status.Conditions) {
					assert.Same(t, &tc.status.Conditions[idx], got)
				}
			}
		})
	}
}
