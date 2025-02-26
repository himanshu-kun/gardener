// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cloudprofile

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/validation"
)

type cloudProfileStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy defines the storage strategy for CloudProfiles.
var Strategy = cloudProfileStrategy{api.Scheme, names.SimpleNameGenerator}

func (cloudProfileStrategy) NamespaceScoped() bool {
	return false
}

func (cloudProfileStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
}

func (cloudProfileStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	cloudprofile := obj.(*core.CloudProfile)
	return validation.ValidateCloudProfileCreation(cloudprofile)
}

func (cloudProfileStrategy) Canonicalize(obj runtime.Object) {
}

func (cloudProfileStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (cloudProfileStrategy) PrepareForUpdate(ctx context.Context, newObj, oldObj runtime.Object) {
	_ = oldObj.(*core.CloudProfile)
	_ = newObj.(*core.CloudProfile)
}

func (cloudProfileStrategy) AllowUnconditionalUpdate() bool {
	return true
}

func (cloudProfileStrategy) ValidateUpdate(ctx context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldProfile, newProfile := oldObj.(*core.CloudProfile), newObj.(*core.CloudProfile)
	return validation.ValidateCloudProfileUpdate(newProfile, oldProfile)
}

// WarningsOnCreate returns warnings to the client performing a create.
func (cloudProfileStrategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	return nil
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (cloudProfileStrategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	return nil
}
