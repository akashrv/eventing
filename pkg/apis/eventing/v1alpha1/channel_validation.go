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
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/knative/pkg/apis"
)

type ChannelValidator interface {
	Validate(c *Channel) *apis.FieldError
}

// ChannelValidator is initialized by the webhook (or the host)
var (
	GlobalChannelValidator ChannelValidator
)

func (c *Channel) Validate() *apis.FieldError {
	// No validations will run if GlobalChannelValidator is not set. Should we panic is such a case?
	if GlobalChannelValidator != nil {
		return GlobalChannelValidator.Validate(c)
	}
	return nil
}

func (current *Channel) CheckImmutableFields(og apis.Immutable) *apis.FieldError {
	if og == nil {
		return nil
	}
	original, ok := og.(*Channel)
	if !ok {
		return &apis.FieldError{Message: "The provided resource was not a Channel"}
	}
	ignoreArguments := cmpopts.IgnoreFields(ChannelSpec{}, "Arguments", "Subscribable")
	if diff := cmp.Diff(original.Spec, current.Spec, ignoreArguments); diff != "" {
		return &apis.FieldError{
			Message: "Immutable fields changed",
			Paths:   []string{"spec.provisioner"},
		}
	}
	return nil
}
