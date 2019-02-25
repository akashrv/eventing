/*
Copyright 2018 The Knative Authors

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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/pkg/apis"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var dnsName = "example.com"

// pkg/webhook/validators/channel_validator_test.go covers all test cases exhaustively. Hence just testing integration with a mock Validator
func TestChannelValidation(t *testing.T) {

	channel := Channel{}
	tests := []struct {
		name                      string
		setGlobalChannelValidator bool
		expected                  *apis.FieldError
	}{
		{
			name:                      "GlobalChannelValidator not set",
			setGlobalChannelValidator: false,
			expected:                  nil,
		},
		{
			name:                      "Test MockValidator",
			setGlobalChannelValidator: true,
			expected:                  &apis.FieldError{Message: "Test Error"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setGlobalChannelValidator {
				GlobalChannelValidator = &MockValidator{}
			} else {
				GlobalChannelValidator = nil
			}
			actual := channel.Validate()
			if diff := cmp.Diff(tc.expected.Error(), actual.Error()); diff != "" {
				t.Errorf("%s: validate (-want, +got) = %v", tc.name, diff)
			}
		})
	}
}

func TestChannelImmutableFields(t *testing.T) {
	tests := []struct {
		name string
		new  apis.Immutable
		old  apis.Immutable
		want *apis.FieldError
	}{{
		name: "good (new)",
		new: &Channel{
			Spec: ChannelSpec{
				Provisioner: &corev1.ObjectReference{
					Name: "foo",
				},
			},
		},
		old:  nil,
		want: nil,
	}, {
		name: "good (no change)",
		new: &Channel{
			Spec: ChannelSpec{
				Provisioner: &corev1.ObjectReference{
					Name: "foo",
				},
			},
		},
		old: &Channel{
			Spec: ChannelSpec{
				Provisioner: &corev1.ObjectReference{
					Name: "foo",
				},
			},
		},
		want: nil,
	}, {
		name: "good (arguments change)",
		new: &Channel{
			Spec: ChannelSpec{
				Provisioner: &corev1.ObjectReference{
					Name: "foo",
				},
				Arguments: &runtime.RawExtension{
					Raw: []byte("\"foo\":\"bar\""),
				},
			},
		},
		old: &Channel{
			Spec: ChannelSpec{
				Provisioner: &corev1.ObjectReference{
					Name: "foo",
				},
				Arguments: &runtime.RawExtension{
					Raw: []byte(`{"foo":"baz"}`),
				},
			},
		},
		want: nil,
	}, {
		name: "bad (not channel)",
		new: &Channel{
			Spec: ChannelSpec{
				Provisioner: &corev1.ObjectReference{
					Name: "foo",
				},
			},
		},
		old: &Subscription{},
		want: &apis.FieldError{
			Message: "The provided resource was not a Channel",
		},
	}, {
		name: "bad (provisioner changes)",
		new: &Channel{
			Spec: ChannelSpec{
				Provisioner: &corev1.ObjectReference{
					Name: "foo",
				},
			},
		},
		old: &Channel{
			Spec: ChannelSpec{
				Provisioner: &corev1.ObjectReference{
					Name: "bar",
				},
			},
		},
		want: &apis.FieldError{
			Message: "Immutable fields changed",
			Paths:   []string{"spec.provisioner"},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.new.CheckImmutableFields(test.old)
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("Validate (-want, +got) = %v", diff)
			}
		})
	}
}

type MockValidator struct {
}

func (mv *MockValidator) Validate(c *Channel) *apis.FieldError {
	return &apis.FieldError{Message: "Test Error"}
}
