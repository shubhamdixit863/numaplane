/*
Copyright 2023.

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

package v1alpha1

import (
	"reflect"
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:validation:Enum="";Pending;Running;Failed;NotApplicable
type GitSyncPhase string

type ConditionType string

const (
	GitSyncPhasePending GitSyncPhase = "Pending"
	GitSyncPhaseRunning GitSyncPhase = "Running"
	GitSyncPhaseFailed  GitSyncPhase = "Failed"
	GitSyncPhaseNA      GitSyncPhase = "NotApplicable" // use this for the case in which this cluster isn't listed as a Destination

	// GitSyncConditionConfigured has the status True when the GitSync
	// has valid configuration.
	GitSyncConditionConfigured ConditionType = "Configured"
)

// GitSyncSpec defines the desired state of GitSync
type GitSyncSpec struct {
	// Important: Run "make" to regenerate code after modifying this file

	// RepositoryPath lists the Git Repository path to watch
	RepositoryPath RepositoryPath `json:"repositoryPath"`

	// Destination describe which cluster/namespace to sync it
	Destination Destination `json:"destination"`
}

// GitSyncStatus defines the observed state of GitSync
type GitSyncStatus struct {
	// Important: Run "make" to regenerate code after modifying this file
	Phase GitSyncPhase `json:"phase,omitempty"`
	// Conditions are the latest available observations of a resource's current state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Message is added if there's a failure
	Message string `json:"message,omitempty"`

	// Last commit processed and the status
	CommitStatus *CommitStatus `json:"commitStatus,omitempty"`
}

// RepositoryPath indicates a particular Git path
type RepositoryPath struct {
	// Name is a unique name
	Name string `json:"name"`

	// RepoUrl is the URL to the repository itself
	RepoUrl string `json:"repoUrl"`

	// Path is the full path from the root of the repository to where the resources are held
	// If Path is empty, then the root directory will be used.
	// Can be a file or a directory
	// Note that all resources within this path (described by .yaml files) will be synced
	Path string `json:"path"`

	// TargetRevision specifies the target revision to sync to, it can be a branch, a tag,
	// or a commit hash.
	TargetRevision string `json:"targetRevision"`
}

// Destination indicates a Cluster to sync to
type Destination struct {
	Cluster string `json:"cluster"`

	// Namespace is optional, as the Resources may be on the cluster level
	// (Note that some Resources describe their namespace within their spec: for those that don't it's useful to have it here)
	Namespace string `json:"namespace,omitempty"`
}

// CommitStatus maintains the status of syncing an individual Git commit
type CommitStatus struct {
	// Hash of the git commit
	Hash string `json:"hash"`

	// Synced indicates if the sync went through
	Synced bool `json:"synced"`

	// SyncTime represents the last time that we attempted to sync this commit (regardless of whether it succeeded)
	SyncTime metav1.Time `json:"syncTime"`

	// Error indicates an error that occurred upon attempting sync, if any
	Error string `json:"error,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// GitSync is the Schema for the gitsyncs API
type GitSync struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GitSyncSpec   `json:"spec"`
	Status GitSyncStatus `json:"status,omitempty"`
}

// String returns the general purpose string representation
func (gitSync GitSync) String() string {
	return gitSync.Namespace + "/" + gitSync.Name
}

//+kubebuilder:object:root=true

// GitSyncList contains a list of GitSync
type GitSyncList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GitSync `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GitSync{}, &GitSyncList{})
}

// ContainsClusterDestination determines if the cluster matches the Destination
func (gitSyncSpec *GitSyncSpec) ContainsClusterDestination(cluster string) bool {
	return gitSyncSpec.Destination.Cluster == cluster
}

// GetDestinationNamespace gets the namespace with the given cluster,
// if not found, then return empty.
func (gitSyncSpec *GitSyncSpec) GetDestinationNamespace(cluster string) string {
	if gitSyncSpec.Destination.Cluster == cluster {
		return gitSyncSpec.Destination.Namespace
	}
	return ""
}

func (status *GitSyncStatus) SetPhase(phase GitSyncPhase, msg string) {
	status.Phase = phase
	status.Message = msg
}

// InitializeConditions initializes the conditions to Unknown
func (status *GitSyncStatus) InitializeConditions(conditionTypes ...ConditionType) {
	for _, t := range conditionTypes {
		c := metav1.Condition{
			Type:   string(t),
			Status: metav1.ConditionUnknown,
			Reason: "Unknown",
		}
		status.setCondition(c)
	}
}

// setCondition sets a Condition, and sorts the list of Conditions
func (status *GitSyncStatus) setCondition(condition metav1.Condition) {
	var conditions []metav1.Condition
	// copy the list of Conditions, and if we find one of this type, replace it and return
	for _, c := range status.Conditions {
		if c.Type != condition.Type {
			conditions = append(conditions, c)
		} else {
			condition.LastTransitionTime = c.LastTransitionTime
			if reflect.DeepEqual(&condition, &c) {
				return
			}
		}
	}
	// didn't find a Condition of this type, so append it to the end of the list, and sort the list for easy read
	condition.LastTransitionTime = metav1.NewTime(time.Now())
	conditions = append(conditions, condition)
	sort.Slice(conditions, func(i, j int) bool { return conditions[i].Type < conditions[j].Type })
	status.Conditions = conditions
}

// InitConditions sets conditions to Unknown state.
func (status *GitSyncStatus) InitConditions() {
	status.InitializeConditions(GitSyncConditionConfigured)
	status.SetPhase(GitSyncPhasePending, "")
}

func (status *GitSyncStatus) markTypeStatus(t ConditionType, s metav1.ConditionStatus, reason, message string) {
	status.setCondition(metav1.Condition{
		Type:    string(t),
		Status:  s,
		Reason:  reason,
		Message: message,
	})
}

// MarkConditionTrue sets the status of t to true
func (status *GitSyncStatus) MarkConditionTrue(t ConditionType) {
	status.markTypeStatus(t, metav1.ConditionTrue, "Successful", "Successful")
}

// MarkConditionFalse sets the status of t to false
func (status *GitSyncStatus) MarkConditionFalse(t ConditionType, reason, message string) {
	status.markTypeStatus(t, metav1.ConditionFalse, reason, message)
}

// MarkConditionUnknown sets the status of t to unknown
func (status *GitSyncStatus) MarkConditionUnknown(t ConditionType, reason, message string) {
	status.markTypeStatus(t, metav1.ConditionUnknown, reason, message)
}

// MarkRunning sets the GitSync to Running
func (status *GitSyncStatus) MarkRunning() {
	status.MarkConditionTrue(GitSyncConditionConfigured)
	status.SetPhase(GitSyncPhaseRunning, "")
}

// MarkFailed sets the GitSync to Failed
func (status *GitSyncStatus) MarkFailed(reason, message string) {
	status.MarkConditionFalse(GitSyncConditionConfigured, reason, message)
	status.SetPhase(GitSyncPhaseFailed, message)
}

// MarkNotApplicable sets the GitSync to Not Applicable
func (status *GitSyncStatus) MarkNotApplicable(reason, message string) {
	status.MarkConditionFalse(GitSyncConditionConfigured, reason, message)
	status.SetPhase(GitSyncPhaseNA, message)
}
