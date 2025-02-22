/*
 * Tencent is pleased to support the open source community by making Blueking Container Service available.
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package v2

import (
	"fmt"

	"github.com/Tencent/bk-bcs/bcs-common/common/types"
	
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// BcsEndpointSpec defines the desired state of BcsEndpoint
type BcsEndpointSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	types.BcsEndpoint
}

// BcsEndpointStatus defines the observed state of BcsEndpoint
type BcsEndpointStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// BcsEndpoint is the Schema for the bcsendpoints API
type BcsEndpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BcsEndpointSpec   `json:"spec,omitempty"`
	Status BcsEndpointStatus `json:"status,omitempty"`
}

func (e *BcsEndpoint) GetUuid() string {
	return fmt.Sprintf("%s.%s", e.Namespace, e.Name)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// BcsEndpointList contains a list of BcsEndpoint
type BcsEndpointList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BcsEndpoint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BcsEndpoint{}, &BcsEndpointList{})
}
