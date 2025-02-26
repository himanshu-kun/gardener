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

// Code generated by client-gen. DO NOT EDIT.

package internalversion

import (
	"context"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"

	core "github.com/gardener/gardener/pkg/apis/core"
	scheme "github.com/gardener/gardener/pkg/client/core/clientset/internalversion/scheme"
)

// SeedsGetter has a method to return a SeedInterface.
// A group's client should implement this interface.
type SeedsGetter interface {
	Seeds() SeedInterface
}

// SeedInterface has methods to work with Seed resources.
type SeedInterface interface {
	Create(ctx context.Context, seed *core.Seed, opts v1.CreateOptions) (*core.Seed, error)
	Update(ctx context.Context, seed *core.Seed, opts v1.UpdateOptions) (*core.Seed, error)
	UpdateStatus(ctx context.Context, seed *core.Seed, opts v1.UpdateOptions) (*core.Seed, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*core.Seed, error)
	List(ctx context.Context, opts v1.ListOptions) (*core.SeedList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *core.Seed, err error)
	SeedExpansion
}

// seeds implements SeedInterface
type seeds struct {
	client rest.Interface
}

// newSeeds returns a Seeds
func newSeeds(c *CoreClient) *seeds {
	return &seeds{
		client: c.RESTClient(),
	}
}

// Get takes name of the seed, and returns the corresponding seed object, and an error if there is any.
func (c *seeds) Get(ctx context.Context, name string, options v1.GetOptions) (result *core.Seed, err error) {
	result = &core.Seed{}
	err = c.client.Get().
		Resource("seeds").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Seeds that match those selectors.
func (c *seeds) List(ctx context.Context, opts v1.ListOptions) (result *core.SeedList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &core.SeedList{}
	err = c.client.Get().
		Resource("seeds").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested seeds.
func (c *seeds) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("seeds").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a seed and creates it.  Returns the server's representation of the seed, and an error, if there is any.
func (c *seeds) Create(ctx context.Context, seed *core.Seed, opts v1.CreateOptions) (result *core.Seed, err error) {
	result = &core.Seed{}
	err = c.client.Post().
		Resource("seeds").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(seed).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a seed and updates it. Returns the server's representation of the seed, and an error, if there is any.
func (c *seeds) Update(ctx context.Context, seed *core.Seed, opts v1.UpdateOptions) (result *core.Seed, err error) {
	result = &core.Seed{}
	err = c.client.Put().
		Resource("seeds").
		Name(seed.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(seed).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *seeds) UpdateStatus(ctx context.Context, seed *core.Seed, opts v1.UpdateOptions) (result *core.Seed, err error) {
	result = &core.Seed{}
	err = c.client.Put().
		Resource("seeds").
		Name(seed.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(seed).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the seed and deletes it. Returns an error if one occurs.
func (c *seeds) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("seeds").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *seeds) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("seeds").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched seed.
func (c *seeds) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *core.Seed, err error) {
	result = &core.Seed{}
	err = c.client.Patch(pt).
		Resource("seeds").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
