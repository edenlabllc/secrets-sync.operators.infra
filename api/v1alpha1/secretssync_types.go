/*
Copyright 2023 @apanasiuk-el edenlabllc.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type SrcSecret struct {
	SrcNamespace string      `json:"srcNamespace"`
	DstSecrets   []DstSecret `json:"dstSecrets,omitempty"`
}

type DstSecret struct {
	Name string            `json:"name,omitempty"`
	Keys map[string]string `json:"keys,omitempty"`
}

// SecretsSyncSpec defines the desired state of SecretsSync
type SecretsSyncSpec struct {
	Secrets map[string]SrcSecret `json:"secrets"`
}

// SecretsSyncStatus defines the observed state of SecretsSync
type SecretsSyncStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	CreatedTime *metav1.Time `json:"createdTime,omitempty"`
	Error       string       `json:"error,omitempty"`
	Phase       string       `json:"phase,omitempty"`
	Count       int          `json:"count,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="PHASE",type=string,JSONPath=`.status.phase`
//+kubebuilder:printcolumn:name="CREATED-TIME",type=string,JSONPath=".status.createdTime"
//+kubebuilder:printcolumn:name="COUNT",type=integer,JSONPath=".status.count"

// SecretsSync is the Schema for the secretssyncs API
type SecretsSync struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SecretsSyncSpec   `json:"spec,omitempty"`
	Status SecretsSyncStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SecretsSyncList contains a list of SecretsSync
type SecretsSyncList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecretsSync `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SecretsSync{}, &SecretsSyncList{})
}
