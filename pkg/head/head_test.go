package head

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
)

func TestNewHeadOptions(t *testing.T) {
	streams := genericclioptions.NewTestIOStreamsDiscard()
	opts := NewHeadOptions(streams)

	if opts.ConfigFlags == nil {
		t.Error("Expected ConfigFlags to be initialized, but it was nil")
	}
	if opts.PrintFlags == nil {
		t.Error("Expected PrintFlags to be initialized, but it was nil")
	}
	if opts.IOStreams.Out == nil {
		t.Error("Expected IOStreams.Out to be initialized, but it was nil")
	}
}

func TestComplete(t *testing.T) {
	streams := genericclioptions.NewTestIOStreamsDiscard()
	opts := NewHeadOptions(streams)

	// Use memory-backed config flags for testing to avoid loading from the filesystem.
	opts.ConfigFlags = genericclioptions.NewConfigFlags(true)
	*opts.ConfigFlags.Namespace = "test"

	// This test requires a valid kubeconfig to run. We assume one is present.
	// To test error conditions, see TestCompleteError.
	err := opts.Complete("pods")
	if err != nil {
		t.Fatalf("unexpected error during Complete: %v", err)
	}

	if opts.DynamicClient == nil {
		t.Error("DynamicClient should have been initialized")
	}
	if opts.Mapper == nil {
		t.Error("Mapper should have been initialized")
	}
	if opts.Namespace != "test" {
		t.Errorf("expected namespace to be 'test', got %q", opts.Namespace)
	}
}

// TestCompleteError tests the error paths in the Complete function.
func TestCompleteError(t *testing.T) {
	streams := genericclioptions.NewTestIOStreamsDiscard()
	opts := NewHeadOptions(streams)

	// Point to a non-existent kubeconfig file to trigger an error.
	opts.ConfigFlags = genericclioptions.NewConfigFlags(true)
	*opts.ConfigFlags.KubeConfig = "/tmp/non-existent-kubeconfig-for-test"

	err := opts.Complete("pods")
	if err == nil {
		t.Fatal("expected an error when using a non-existent kubeconfig, but got none")
	}
	if !strings.Contains(err.Error(), "non-existent-kubeconfig-for-test") {
		t.Errorf("expected error to mention the invalid path, but got: %v", err)
	}
}

func TestValidate(t *testing.T) {
	testCases := []struct {
		name          string
		opts          *HeadOptions
		expectedError string
	}{
		{
			name: "valid options",
			opts: &HeadOptions{
				Limit:         10,
				Interactive:   false,
				ContinueToken: "",
				PrintFlags:    genericclioptions.NewPrintFlags(""),
			},
			expectedError: "",
		},
		{
			name: "invalid limit",
			opts: &HeadOptions{
				Limit: 0,
			},
			expectedError: "--limit must be a positive number",
		},
		{
			name: "interactive and continue token together",
			opts: &HeadOptions{
				Limit:         10,
				Interactive:   true,
				ContinueToken: "token",
			},
			expectedError: "cannot use --interactive and --continue flags together",
		},
		{
			name: "interactive with json output",
			opts: &HeadOptions{
				Limit:       10,
				Interactive: true,
				PrintFlags:  genericclioptions.NewPrintFlags("").WithDefaultOutput("json"),
			},
			expectedError: "interactive mode is only supported for standard and wide table output",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.opts.Validate()
			if err != nil && tc.expectedError == "" {
				t.Errorf("Unexpected error: %v", err)
			}
			if err == nil && tc.expectedError != "" {
				t.Errorf("Expected error %q, but got none", tc.expectedError)
			}
			if err != nil && tc.expectedError != "" && err.Error() != tc.expectedError {
				t.Errorf("Expected error %q, but got %q", tc.expectedError, err.Error())
			}
		})
	}
}

func TestNewRestClient(t *testing.T) {
	testCases := []struct {
		name        string
		gv          schema.GroupVersion
		expectedAPI string
	}{
		{
			name:        "core group",
			gv:          schema.GroupVersion{Group: "", Version: "v1"},
			expectedAPI: "/api",
		},
		{
			name:        "apps group",
			gv:          schema.GroupVersion{Group: "apps", Version: "v1"},
			expectedAPI: "/apis",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := NewRestClient(rest.Config{}, tc.gv)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if client == nil {
				t.Fatal("rest client should not be nil")
			}
			// This is a bit of a hack to check the API path, as the client doesn't expose it directly.
			if !strings.Contains(client.Get().URL().Path, tc.expectedAPI) {
				t.Errorf("expected API path to contain %q, but it did not", tc.expectedAPI)
			}
		})
	}
}

func TestGetResourceGVR(t *testing.T) {
	streams := genericclioptions.NewTestIOStreamsDiscard()
	opts := NewHeadOptions(streams)

	testCases := []struct {
		name          string
		resourceArg   string
		mapper        meta.RESTMapper
		expectedGVR   schema.GroupVersionResource
		expectedError string
	}{
		{
			name:        "simple resource",
			resourceArg: "pods",
			mapper: &fakeRESTMapperImpl{
				gvr: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			},
			expectedGVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		},
		{
			name:        "resource with group",
			resourceArg: "deployments.apps",
			mapper: &fakeRESTMapperImpl{
				gvr: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			},
			expectedGVR: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
		},
		{
			name:        "resource not found",
			resourceArg: "nonexistent",
			mapper: &fakeRESTMapperImpl{
				err: errors.New("not found"),
			},
			expectedError: `the server doesn't have a resource type "nonexistent"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts.Resource = tc.resourceArg
			opts.Mapper = tc.mapper
			gvr, err := opts.GetResourceGVR()

			if err != nil && tc.expectedError == "" {
				t.Errorf("unexpected error: %v", err)
			}
			if err == nil && tc.expectedError != "" {
				t.Errorf("expected error %q, but got none", tc.expectedError)
			}
			if err != nil && tc.expectedError != "" && err.Error() != tc.expectedError {
				t.Errorf("expected error %q, but got %q", tc.expectedError, err.Error())
			}
			if err == nil && gvr != tc.expectedGVR {
				t.Errorf("expected gvr %v, got %v", tc.expectedGVR, gvr)
			}
		})
	}
}

// fakeRESTMapper returns a basic RESTMapper for testing.
func fakeRESTMapper() meta.RESTMapper {
	return &fakeRESTMapperImpl{
		gvr: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
	}
}

type fakeRESTMapperImpl struct {
	gvr schema.GroupVersionResource
	err error
}

func (f *fakeRESTMapperImpl) KindFor(resource schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, fmt.Errorf("not implemented")
}

func (f *fakeRESTMapperImpl) KindsFor(resource schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *fakeRESTMapperImpl) ResourceFor(input schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	if f.err != nil {
		return schema.GroupVersionResource{}, f.err
	}
	// If the input has a group, make sure it matches.
	if input.Group != "" && input.Group != f.gvr.Group {
		return schema.GroupVersionResource{}, errors.New("group does not match")
	}
	return f.gvr, nil
}

func (f *fakeRESTMapperImpl) ResourcesFor(input schema.GroupVersionResource) ([]schema.GroupVersionResource, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *fakeRESTMapperImpl) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *fakeRESTMapperImpl) RESTMappings(gk schema.GroupKind, versions ...string) ([]*meta.RESTMapping, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *fakeRESTMapperImpl) ResourceSingularizer(resource string) (string, error) {
	return "", fmt.Errorf("not implemented")
}
