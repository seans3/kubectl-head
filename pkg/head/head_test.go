package head

import (
	"fmt"
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
	gv := schema.GroupVersion{Group: "apps", Version: "v1"}
	restClient, err := NewRestClient(rest.Config{}, gv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if restClient == nil {
		t.Fatal("rest client should not be nil")
	}
}

func TestGetResourceGVR(t *testing.T) {
	streams := genericclioptions.NewTestIOStreamsDiscard()
	opts := NewHeadOptions(streams)
	opts.Resource = "pods"
	opts.Mapper = &fakeRESTMapperImpl{
		gvr: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
	}

	gvr, err := opts.GetResourceGVR()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	if gvr != expectedGVR {
		t.Errorf("expected gvr %v, got %v", expectedGVR, gvr)
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
}

func (f *fakeRESTMapperImpl) KindFor(resource schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, fmt.Errorf("not implemented")
}

func (f *fakeRESTMapperImpl) KindsFor(resource schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *fakeRESTMapperImpl) ResourceFor(input schema.GroupVersionResource) (schema.GroupVersionResource, error) {
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