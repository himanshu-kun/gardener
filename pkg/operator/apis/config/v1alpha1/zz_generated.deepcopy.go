//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright (c) SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

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

// Code generated by deepcopy-gen. DO NOT EDIT.

package v1alpha1

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"

	configv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControllerConfiguration) DeepCopyInto(out *ControllerConfiguration) {
	*out = *in
	in.Garden.DeepCopyInto(&out.Garden)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControllerConfiguration.
func (in *ControllerConfiguration) DeepCopy() *ControllerConfiguration {
	if in == nil {
		return nil
	}
	out := new(ControllerConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GardenControllerConfig) DeepCopyInto(out *GardenControllerConfig) {
	*out = *in
	if in.ConcurrentSyncs != nil {
		in, out := &in.ConcurrentSyncs, &out.ConcurrentSyncs
		*out = new(int)
		**out = **in
	}
	if in.SyncPeriod != nil {
		in, out := &in.SyncPeriod, &out.SyncPeriod
		*out = new(v1.Duration)
		**out = **in
	}
	if in.ETCDConfig != nil {
		in, out := &in.ETCDConfig, &out.ETCDConfig
		*out = new(configv1alpha1.ETCDConfig)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GardenControllerConfig.
func (in *GardenControllerConfig) DeepCopy() *GardenControllerConfig {
	if in == nil {
		return nil
	}
	out := new(GardenControllerConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OperatorConfiguration) DeepCopyInto(out *OperatorConfiguration) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.RuntimeClientConnection = in.RuntimeClientConnection
	in.LeaderElection.DeepCopyInto(&out.LeaderElection)
	in.Server.DeepCopyInto(&out.Server)
	if in.Debugging != nil {
		in, out := &in.Debugging, &out.Debugging
		*out = new(componentbaseconfigv1alpha1.DebuggingConfiguration)
		(*in).DeepCopyInto(*out)
	}
	if in.FeatureGates != nil {
		in, out := &in.FeatureGates, &out.FeatureGates
		*out = make(map[string]bool, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	in.Controllers.DeepCopyInto(&out.Controllers)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OperatorConfiguration.
func (in *OperatorConfiguration) DeepCopy() *OperatorConfiguration {
	if in == nil {
		return nil
	}
	out := new(OperatorConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *OperatorConfiguration) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Server) DeepCopyInto(out *Server) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Server.
func (in *Server) DeepCopy() *Server {
	if in == nil {
		return nil
	}
	out := new(Server)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServerConfiguration) DeepCopyInto(out *ServerConfiguration) {
	*out = *in
	out.Webhooks = in.Webhooks
	if in.HealthProbes != nil {
		in, out := &in.HealthProbes, &out.HealthProbes
		*out = new(Server)
		**out = **in
	}
	if in.Metrics != nil {
		in, out := &in.Metrics, &out.Metrics
		*out = new(Server)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServerConfiguration.
func (in *ServerConfiguration) DeepCopy() *ServerConfiguration {
	if in == nil {
		return nil
	}
	out := new(ServerConfiguration)
	in.DeepCopyInto(out)
	return out
}
