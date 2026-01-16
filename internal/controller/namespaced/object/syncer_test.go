package object

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestHasSpecFields(t *testing.T) {
	tests := []struct {
		name     string
		mfe      metav1.ManagedFieldsEntry
		expected bool
	}{
		{
			name: "has spec fields",
			mfe: metav1.ManagedFieldsEntry{
				Manager:   "test-manager",
				Operation: metav1.ManagedFieldsOperationUpdate,
				FieldsV1:  &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:credentials":{}}}`)},
			},
			expected: true,
		},
		{
			name: "only metadata fields (finalizers)",
			mfe: metav1.ManagedFieldsEntry{
				Manager:   "test-manager",
				Operation: metav1.ManagedFieldsOperationUpdate,
				FieldsV1:  &metav1.FieldsV1{Raw: []byte(`{"f:metadata":{"f:finalizers":{}}}`)},
			},
			expected: false,
		},
		{
			name: "nil FieldsV1",
			mfe: metav1.ManagedFieldsEntry{
				Manager:   "test-manager",
				Operation: metav1.ManagedFieldsOperationUpdate,
				FieldsV1:  nil,
			},
			expected: false,
		},
		{
			name: "empty FieldsV1 Raw",
			mfe: metav1.ManagedFieldsEntry{
				Manager:   "test-manager",
				Operation: metav1.ManagedFieldsOperationUpdate,
				FieldsV1:  &metav1.FieldsV1{Raw: []byte{}},
			},
			expected: false,
		},
		{
			name: "invalid JSON - conservative fallback",
			mfe: metav1.ManagedFieldsEntry{
				Manager:   "test-manager",
				Operation: metav1.ManagedFieldsOperationUpdate,
				FieldsV1:  &metav1.FieldsV1{Raw: []byte(`not valid json`)},
			},
			expected: true,
		},
		{
			name: "has both metadata and spec fields",
			mfe: metav1.ManagedFieldsEntry{
				Manager:   "test-manager",
				Operation: metav1.ManagedFieldsOperationUpdate,
				FieldsV1:  &metav1.FieldsV1{Raw: []byte(`{"f:metadata":{"f:labels":{}},"f:spec":{"f:data":{}}}`)},
			},
			expected: true,
		},
		{
			name: "only status fields",
			mfe: metav1.ManagedFieldsEntry{
				Manager:   "test-manager",
				Operation: metav1.ManagedFieldsOperationUpdate,
				FieldsV1:  &metav1.FieldsV1{Raw: []byte(`{"f:status":{"f:conditions":{}}}`)},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasSpecFields(tt.mfe)
			if result != tt.expected {
				t.Errorf("hasSpecFields() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestNeedSSAFieldManagerUpgrade(t *testing.T) {
	legacyManagers := sets.New[string]("crossplane-kubernetes-provider", "legacy-manager")

	tests := []struct {
		name          string
		accessor      metav1.Object
		expected      bool
	}{
		{
			name:     "nil accessor",
			accessor: nil,
			expected: false,
		},
		{
			name: "legacy manager with spec fields - needs migration",
			accessor: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"managedFields": []interface{}{
							map[string]interface{}{
								"manager":   "crossplane-kubernetes-provider",
								"operation": "Update",
								"fieldsV1":  map[string]interface{}{"f:spec": map[string]interface{}{"f:credentials": map[string]interface{}{}}},
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "legacy manager with data fields (spec-less) - needs migration",
			accessor: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"managedFields": []interface{}{
							map[string]interface{}{
								"manager":   "crossplane-kubernetes-provider",
								"operation": "Update",
								"fieldsV1":  map[string]interface{}{"f:data": map[string]interface{}{"f:key": map[string]interface{}{}}},
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "legacy manager with only metadata fields - no migration",
			accessor: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"managedFields": []interface{}{
							map[string]interface{}{
								"manager":   "crossplane-kubernetes-provider",
								"operation": "Update",
								"fieldsV1":  map[string]interface{}{"f:metadata": map[string]interface{}{"f:finalizers": map[string]interface{}{}}},
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "non-legacy manager with spec fields - no migration",
			accessor: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"managedFields": []interface{}{
							map[string]interface{}{
								"manager":   "some-other-manager",
								"operation": "Update",
								"fieldsV1":  map[string]interface{}{"f:spec": map[string]interface{}{"f:data": map[string]interface{}{}}},
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "legacy manager with Apply operation - no migration",
			accessor: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"managedFields": []interface{}{
							map[string]interface{}{
								"manager":   "crossplane-kubernetes-provider",
								"operation": "Apply",
								"fieldsV1":  map[string]interface{}{"f:spec": map[string]interface{}{"f:data": map[string]interface{}{}}},
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "empty managed fields",
			accessor: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"managedFields": []interface{}{},
					},
				},
			},
			expected: false,
		},
	}

	syncer := &SSAResourceSyncer{
		legacyCSAFieldManagers: legacyManagers,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := syncer.needSSAFieldManagerUpgrade(tt.accessor)
			if result != tt.expected {
				t.Errorf("needSSAFieldManagerUpgrade() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
