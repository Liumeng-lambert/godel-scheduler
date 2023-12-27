/*
Copyright 2023 The Godel Scheduler Authors.

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

package api

import (
	"fmt"
	"time"

	schedulingv1a1 "github.com/kubewharf/godel-scheduler-api/pkg/apis/scheduling/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kubewharf/godel-scheduler/pkg/util"
	podutil "github.com/kubewharf/godel-scheduler/pkg/util/pod"
)

type StoredUnit interface {
	GetPods() []*QueuedPodInfo
	NumPods() int
	GetKey() string
}

// PodGroupUnit is the podGroup Unit implementation
type PodGroupUnit struct {
	key              string
	podGroup         *schedulingv1a1.PodGroup
	priority         int32
	queuedPodInfoMap map[string]*QueuedPodInfo
	timestamp        time.Time

	// be used by metrics and tracing
	// it's generated by calling GetUnitProperty()
	unitProperty UnitProperty
}

var (
	_ ScheduleUnit   = &PodGroupUnit{}
	_ ObservableUnit = &PodGroupUnit{}
)

func keyForPodGroupUnit(podGroup *schedulingv1a1.PodGroup) string {
	if podGroup == nil {
		return ""
	}
	return string(PodGroupUnitType) + "/" + podGroup.Namespace + "/" + podGroup.Name
}

func NewPodGroupUnit(podGroup *schedulingv1a1.PodGroup, priority int32) *PodGroupUnit {
	return &PodGroupUnit{
		key:              keyForPodGroupUnit(podGroup),
		podGroup:         podGroup,
		priority:         priority,
		queuedPodInfoMap: make(map[string]*QueuedPodInfo),
		timestamp:        time.Now(),
	}
}

func (p *PodGroupUnit) GetNamespace() string {
	if p.podGroup == nil {
		return ""
	}
	return p.podGroup.Namespace
}

func (p *PodGroupUnit) GetName() string {
	if p.podGroup == nil {
		return ""
	}
	return p.podGroup.Name
}

func (p *PodGroupUnit) GetKey() string {
	return p.key
}

func (p *PodGroupUnit) Type() ScheduleUnitType {
	return PodGroupUnitType
}

func (p *PodGroupUnit) ReadyToBePopulated() bool {
	if p.podGroup == nil {
		return false
	}

	return len(p.queuedPodInfoMap) >= int(p.podGroup.Spec.MinMember)
}

func (p *PodGroupUnit) PodBelongToUnit(pod *v1.Pod) bool {
	name := pod.Annotations[podutil.PodGroupNameAnnotationKey]
	if len(name) == 0 || p.podGroup == nil {
		return false
	}

	if p.podGroup.Name == name && p.podGroup.Namespace == pod.Namespace {
		return true
	}

	return false
}

func (p *PodGroupUnit) GetCreationTimestamp() *metav1.Time {
	return &p.podGroup.CreationTimestamp
}

func (p *PodGroupUnit) GetEnqueuedTimestamp() *metav1.Time {
	return &metav1.Time{Time: p.timestamp}
}

func (p *PodGroupUnit) SetEnqueuedTimeStamp(ts time.Time) {
	for _, podInfo := range p.queuedPodInfoMap {
		podInfo.Timestamp = ts
	}
	p.timestamp = ts
}

// GetPriority return pod group unit priority. CreatUnit already assigns the value, it's safe to return it directly.
func (p *PodGroupUnit) GetPriority() int32 {
	return p.priority
}

func (p *PodGroupUnit) ValidatePodCount(podCount int) bool {
	return int32(podCount) >= p.podGroup.Spec.MinMember
}

// If iterating the map is a performance concern here, we can introduce more complex data structure.
func (p *PodGroupUnit) GetPods() []*QueuedPodInfo {
	values := make([]*QueuedPodInfo, 0)
	for _, v := range p.queuedPodInfoMap {
		values = append(values, v)
	}
	return values
}

func (p *PodGroupUnit) NumPods() int {
	return len(p.queuedPodInfoMap)
}

func (p *PodGroupUnit) GetPod(pod *QueuedPodInfo) *QueuedPodInfo {
	if pod.Pod == nil {
		return nil
	}
	if pInfo, ok := p.queuedPodInfoMap[string(pod.Pod.UID)]; ok {
		return pInfo
	}
	return nil
}

func (p *PodGroupUnit) AddPod(pod *QueuedPodInfo) error {
	if pod.Pod == nil {
		return fmt.Errorf("invalid pod")
	}
	p.queuedPodInfoMap[string(pod.Pod.UID)] = pod
	return nil
}

func (p *PodGroupUnit) AddPods(pods []*QueuedPodInfo) error {
	for _, pod := range pods {
		if pod.Pod == nil {
			continue
		}
		p.queuedPodInfoMap[string(pod.Pod.UID)] = pod
	}
	return nil
}

func (p *PodGroupUnit) UpdatePod(pod *QueuedPodInfo) error {
	if pod.Pod == nil {
		return fmt.Errorf("invalid pod")
	}
	p.queuedPodInfoMap[string(pod.Pod.UID)] = pod
	return nil
}

func (p *PodGroupUnit) DeletePod(pod *QueuedPodInfo) error {
	if pod.Pod == nil {
		return fmt.Errorf("invalid pod")
	}
	delete(p.queuedPodInfoMap, string(pod.Pod.UID))
	return nil
}

func (p *PodGroupUnit) GetTimeoutPeriod() int32 {
	if p.podGroup.Spec.ScheduleTimeoutSeconds == nil {
		return 300
	} else {
		return *p.podGroup.Spec.ScheduleTimeoutSeconds
	}
}

func (p *PodGroupUnit) GetAnnotations() map[string]string {
	if p.podGroup == nil {
		return map[string]string{}
	}
	return p.podGroup.Annotations
}

func (p *PodGroupUnit) GetMinMember() (int, error) {
	if p.podGroup == nil {
		return -1, fmt.Errorf("pod group is nil")
	}

	return int(p.podGroup.Spec.MinMember), nil
}

// GetRequiredAffinity returns affinity rules specified in PodGroupAffinity.Required
func (p *PodGroupUnit) GetRequiredAffinity() ([]UnitAffinityTerm, error) {
	if p.podGroup.Spec.Affinity == nil ||
		p.podGroup.Spec.Affinity.PodGroupAffinity == nil {
		return nil, nil
	}
	affinity := p.podGroup.Spec.Affinity.PodGroupAffinity
	var terms []UnitAffinityTerm
	for _, term := range affinity.Required {
		if term.TopologyKey == "" {
			continue
		}
		terms = append(terms, UnitAffinityTerm{
			TopologyKey: term.TopologyKey,
		})
	}
	return terms, nil
}

// GetPreferredAffinity returns affinity rules specified in PodGroupAffinity.Preferred
func (p *PodGroupUnit) GetPreferredAffinity() ([]UnitAffinityTerm, error) {
	if p.podGroup.Spec.Affinity == nil ||
		p.podGroup.Spec.Affinity.PodGroupAffinity == nil {
		return nil, nil
	}
	affinity := p.podGroup.Spec.Affinity.PodGroupAffinity
	var terms []UnitAffinityTerm
	for _, term := range affinity.Preferred {
		if term.TopologyKey == "" {
			continue
		}
		terms = append(terms, UnitAffinityTerm{
			TopologyKey: term.TopologyKey,
		})
	}
	return terms, nil
}

func (p *PodGroupUnit) GetAffinityNodeSelector() (*v1.NodeSelector, error) {
	if p.podGroup == nil {
		return nil, fmt.Errorf("empty podGroup in PodGroupUnit %v", p.key)
	}
	if p.podGroup.Spec.Affinity == nil || p.podGroup.Spec.Affinity.PodGroupAffinity == nil {
		return nil, nil
	}
	podGroupAffinity := p.podGroup.Spec.Affinity.PodGroupAffinity
	return podGroupAffinity.NodeSelector, nil
}

func (p *PodGroupUnit) GetSortRulesForAffinity() ([]SortRule, error) {
	if p.podGroup.Spec.Affinity == nil ||
		p.podGroup.Spec.Affinity.PodGroupAffinity == nil {
		return nil, nil
	}
	var rules []SortRule
	for _, r := range p.podGroup.Spec.Affinity.PodGroupAffinity.SortRules {
		// TODO: We set the value of `Dimension` to `Capacity` for backward compatibility. It is expected to be removed in the near future.
		var dimension schedulingv1a1.SortDimension
		if len(string(r.Dimension)) == 0 {
			dimension = schedulingv1a1.Capacity
		} else {
			dimension = r.Dimension
		}
		rules = append(rules, SortRule{
			Resource:  SortResource(r.Resource),
			Dimension: SortDimension(dimension),
			Order:     SortOrder(r.Order),
		})
	}
	return rules, nil
}

func (p *PodGroupUnit) IsDebugModeOn() bool {
	if p.podGroup == nil || p.podGroup.Annotations == nil {
		return false
	}
	debugMode, ok := p.podGroup.Annotations[util.DebugModeAnnotationKey]
	return ok && debugMode == util.DebugModeOn
}

func (p *PodGroupUnit) String() string {
	var pods string
	for k, v := range p.queuedPodInfoMap {
		pods += fmt.Sprintf("%s:%+v,", k, v)
	}
	if len(pods) == 0 {
		pods = "empty"
	}
	return fmt.Sprintf("{Pod:[%s], PodGroup:%+v, Priority:%v, TimeStamp:%v}", pods, p.podGroup, p.priority, p.timestamp)
}

func (p *PodGroupUnit) GetUnitProperty() UnitProperty {
	if p.unitProperty != nil {
		return p.unitProperty
	}

	property, err := NewScheduleUnitProperty(p)
	if err != nil {
		return nil
	}
	p.unitProperty = property
	return p.unitProperty
}

func (p *PodGroupUnit) ResetPods() {
	p.queuedPodInfoMap = make(map[string]*QueuedPodInfo)
}

type SinglePodUnit struct {
	// key is the identifier of scheduling unit, format is "SinglePodUnit/namespace/podname",
	// it's intended to store the key instead of generating it every time to reduce the memory cost.
	key string
	Pod *QueuedPodInfo

	unitProperty UnitProperty
}

var (
	_ ScheduleUnit   = &SinglePodUnit{}
	_ ObservableUnit = &SinglePodUnit{}
)

func keyForSinglePodUnit(pod *v1.Pod) string {
	if pod == nil {
		return ""
	}
	return string(SinglePodUnitType) + "/" + pod.Namespace + "/" + pod.Name
}

func NewSinglePodUnit(podInfo *QueuedPodInfo) *SinglePodUnit {
	var key string
	if podInfo != nil {
		key = keyForSinglePodUnit(podInfo.Pod)
	}

	return &SinglePodUnit{
		Pod: podInfo,
		key: key,
	}
}

func (s *SinglePodUnit) GetName() string {
	if s.Pod == nil || s.Pod.Pod == nil {
		return ""
	}
	return s.Pod.Pod.GetName()
}

func (s *SinglePodUnit) GetNamespace() string {
	if s.Pod == nil || s.Pod.Pod == nil {
		return ""
	}
	return s.Pod.Pod.GetNamespace()
}

func (s *SinglePodUnit) GetKey() string {
	if s.Pod == nil {
		return ""
	}
	return s.key
}

func (s *SinglePodUnit) Type() ScheduleUnitType {
	return SinglePodUnitType
}

func (s *SinglePodUnit) ReadyToBePopulated() bool {
	return s.Pod != nil && s.Pod.Pod != nil
}

func (s *SinglePodUnit) ValidatePodCount(podCnt int) bool {
	return podCnt > 0
}

func (s *SinglePodUnit) PodBelongToUnit(pod *v1.Pod) bool {
	return pod.Namespace == s.Pod.Pod.Namespace && pod.Name == s.Pod.Pod.Name
}

func (s *SinglePodUnit) GetCreationTimestamp() *metav1.Time {
	if s.Pod == nil {
		return nil
	}
	return &s.Pod.Pod.CreationTimestamp
}

func (s *SinglePodUnit) GetEnqueuedTimestamp() *metav1.Time {
	if s.Pod == nil {
		return &metav1.Time{Time: time.Now()}
	}
	return &metav1.Time{Time: s.Pod.Timestamp}
}

func (s *SinglePodUnit) SetEnqueuedTimeStamp(ts time.Time) {
	if s.Pod != nil {
		s.Pod.Timestamp = ts
	}
}

func (s *SinglePodUnit) GetPriority() int32 {
	// pc := s.Pod.Pod.Spec.PriorityClassName
	// if len(pc) == 0 {
	// 	// TODO(yuquan.ren@): Change to use a generic util func which takes Qos, Agent into consideration to retrieve default priority scores.
	// 	// TODO(jiaxin.shan@): 100 is a temporary value used here. This should be updated before going online.
	// 	return int32(100)
	// }
	if s.Pod == nil || s.Pod.Pod == nil || s.Pod.Pod.Spec.Priority == nil {
		return 100
	}

	return *s.Pod.Pod.Spec.Priority
}

// If iterating the map is a performance concern here, we can introduce more complex data structure.
func (s *SinglePodUnit) GetPods() []*QueuedPodInfo {
	if s.Pod == nil {
		return nil
	}
	return []*QueuedPodInfo{s.Pod}
}

func (s *SinglePodUnit) NumPods() int {
	if s.Pod != nil && s.Pod.Pod != nil {
		return 1
	}
	return 0
}

func (s *SinglePodUnit) GetPod(pod *QueuedPodInfo) *QueuedPodInfo {
	if pod.Pod == nil || s.Pod == nil || s.Pod.Pod.UID != pod.Pod.UID {
		return nil
	}
	return s.Pod
}

func (s *SinglePodUnit) AddPod(pod *QueuedPodInfo) error {
	if pod.Pod == nil {
		return fmt.Errorf("invalid pod")
	}
	s.Pod = pod
	s.key = keyForSinglePodUnit(pod.Pod)
	return nil
}

func (s *SinglePodUnit) AddPods(pods []*QueuedPodInfo) error {
	if len(pods) != 1 {
		return fmt.Errorf("invalid pods")
	}
	return nil
}

func (s *SinglePodUnit) UpdatePod(pod *QueuedPodInfo) error {
	if pod.Pod == nil {
		return fmt.Errorf("invalid pod")
	}
	s.Pod = pod
	s.key = keyForSinglePodUnit(pod.Pod)
	return nil
}

func (s *SinglePodUnit) DeletePod(pod *QueuedPodInfo) error {
	if pod.Pod == nil {
		return fmt.Errorf("invalid pod")
	}

	s.Pod = nil

	return nil
}

func (s *SinglePodUnit) GetTimeoutPeriod() int32 {
	return 0
}

func (s *SinglePodUnit) GetAnnotations() map[string]string {
	if s.Pod == nil || s.Pod.Pod == nil {
		return map[string]string{}
	}
	return s.Pod.Pod.Annotations
}

func (s *SinglePodUnit) GetMinMember() (int, error) {
	if s.Pod == nil {
		return -1, fmt.Errorf("pod is nil")
	}
	return 1, nil
}

func (s *SinglePodUnit) GetRequiredAffinity() ([]UnitAffinityTerm, error) {
	return nil, nil
}

func (s *SinglePodUnit) GetAffinityNodeSelector() (*v1.NodeSelector, error) {
	return nil, nil
}

func (s *SinglePodUnit) GetPreferredAffinity() ([]UnitAffinityTerm, error) {
	return nil, nil
}

func (s *SinglePodUnit) GetSortRulesForAffinity() ([]SortRule, error) {
	return nil, nil
}

func (s *SinglePodUnit) IsDebugModeOn() bool {
	if s.Pod == nil || s.Pod.Pod == nil || s.Pod.Pod.Annotations == nil {
		return false
	}

	debugMode, ok := s.Pod.Pod.Annotations[util.DebugModeAnnotationKey]
	return ok && debugMode == util.DebugModeOn
}

func (s *SinglePodUnit) String() string {
	if s.Pod == nil || s.Pod.Pod == nil {
		return "{PodInfo:[empty]}"
	}
	return fmt.Sprintf("{PodInfo:[%s:%+v]}", s.Pod.Pod.UID, s.Pod)
}

func (s *SinglePodUnit) GetUnitProperty() UnitProperty {
	if s.unitProperty != nil {
		return s.unitProperty
	}

	property, err := NewScheduleUnitProperty(s)
	if err != nil {
		return nil
	}
	s.unitProperty = property
	return s.unitProperty
}

func (s *SinglePodUnit) ResetPods() {
	s.Pod = nil
}