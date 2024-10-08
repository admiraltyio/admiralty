/*
 * Copyright The Multicluster-Scheduler Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	"time"

	v1alpha1 "admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	scheme "admiralty.io/multicluster-scheduler/pkg/generated/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// PodChaperonsGetter has a method to return a PodChaperonInterface.
// A group's client should implement this interface.
type PodChaperonsGetter interface {
	PodChaperons(namespace string) PodChaperonInterface
}

// PodChaperonInterface has methods to work with PodChaperon resources.
type PodChaperonInterface interface {
	Create(ctx context.Context, podChaperon *v1alpha1.PodChaperon, opts v1.CreateOptions) (*v1alpha1.PodChaperon, error)
	Update(ctx context.Context, podChaperon *v1alpha1.PodChaperon, opts v1.UpdateOptions) (*v1alpha1.PodChaperon, error)
	UpdateStatus(ctx context.Context, podChaperon *v1alpha1.PodChaperon, opts v1.UpdateOptions) (*v1alpha1.PodChaperon, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.PodChaperon, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.PodChaperonList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.PodChaperon, err error)
	PodChaperonExpansion
}

// podChaperons implements PodChaperonInterface
type podChaperons struct {
	client rest.Interface
	ns     string
}

// newPodChaperons returns a PodChaperons
func newPodChaperons(c *MulticlusterV1alpha1Client, namespace string) *podChaperons {
	return &podChaperons{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the podChaperon, and returns the corresponding podChaperon object, and an error if there is any.
func (c *podChaperons) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.PodChaperon, err error) {
	result = &v1alpha1.PodChaperon{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("podchaperons").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of PodChaperons that match those selectors.
func (c *podChaperons) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.PodChaperonList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.PodChaperonList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("podchaperons").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested podChaperons.
func (c *podChaperons) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("podchaperons").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a podChaperon and creates it.  Returns the server's representation of the podChaperon, and an error, if there is any.
func (c *podChaperons) Create(ctx context.Context, podChaperon *v1alpha1.PodChaperon, opts v1.CreateOptions) (result *v1alpha1.PodChaperon, err error) {
	result = &v1alpha1.PodChaperon{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("podchaperons").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(podChaperon).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a podChaperon and updates it. Returns the server's representation of the podChaperon, and an error, if there is any.
func (c *podChaperons) Update(ctx context.Context, podChaperon *v1alpha1.PodChaperon, opts v1.UpdateOptions) (result *v1alpha1.PodChaperon, err error) {
	result = &v1alpha1.PodChaperon{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("podchaperons").
		Name(podChaperon.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(podChaperon).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *podChaperons) UpdateStatus(ctx context.Context, podChaperon *v1alpha1.PodChaperon, opts v1.UpdateOptions) (result *v1alpha1.PodChaperon, err error) {
	result = &v1alpha1.PodChaperon{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("podchaperons").
		Name(podChaperon.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(podChaperon).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the podChaperon and deletes it. Returns an error if one occurs.
func (c *podChaperons) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("podchaperons").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *podChaperons) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("podchaperons").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched podChaperon.
func (c *podChaperons) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.PodChaperon, err error) {
	result = &v1alpha1.PodChaperon{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("podchaperons").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
