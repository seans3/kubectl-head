package head

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

const (
	// DefaultHeadLimit is the default number of items to return per page.
	DefaultHeadLimit int64 = 10
)

// HeadOptions provides the options and dependencies for the head command.
type HeadOptions struct {
	ConfigFlags *genericclioptions.ConfigFlags
	PrintFlags  *genericclioptions.PrintFlags

	// User-provided resource type (e.g., "pods", "deployments.apps").
	Resource string

	// Flags for the head command.
	Limit         int64
	ContinueToken string
	Interactive   bool
	Selector      string
	AllNamespaces bool

	// Calculated values.
	Namespace     string
	DynamicClient dynamic.Interface
	Mapper        meta.RESTMapper
	RESTConfig    *rest.Config

	genericclioptions.IOStreams
}

// NewHeadOptions returns a new instance of HeadOptions with default values.
func NewHeadOptions(streams genericclioptions.IOStreams) *HeadOptions {
	return &HeadOptions{
		ConfigFlags: genericclioptions.NewConfigFlags(true),
		PrintFlags:  genericclioptions.NewPrintFlags("").WithTypeSetter(scheme.Scheme),
		IOStreams:   streams,
	}
}

// Complete sets all information required for processing the command.
func (o *HeadOptions) Complete(resource string) error {
	var err error
	o.Resource = resource

	// Create a RESTMapper to map resource names (like "pods") to GVRs.
	o.Mapper, err = o.ConfigFlags.ToRESTMapper()
	if err != nil {
		return err
	}

	// Get the namespace from the flags.
	o.Namespace, _, err = o.ConfigFlags.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	// Create a dynamic client that can work with any resource type.
	o.RESTConfig, err = o.ConfigFlags.ToRESTConfig()
	if err != nil {
		return err
	}
	o.DynamicClient, err = dynamic.NewForConfig(o.RESTConfig)
	if err != nil {
		return err
	}

	return nil
}

// Validate ensures that all required arguments and flag values are provided and valid.
func (o *HeadOptions) Validate() error {
	if o.Limit <= 0 {
		return fmt.Errorf("--limit must be a positive number")
	}
	if o.Interactive && o.ContinueToken != "" {
		return fmt.Errorf("cannot use --interactive and --continue flags together")
	}
	// Interactive mode doesn't make sense if the output is not for a human.
	if o.Interactive && (*o.PrintFlags.OutputFormat != "" && *o.PrintFlags.OutputFormat != "wide") {
		return fmt.Errorf("interactive mode is only supported for standard and wide table output")
	}
	return nil
}

// Run executes the head command logic.
func (o *HeadOptions) Run() error {
	gvr, err := o.GetResourceGVR()
	if err != nil {
		return err
	}

	ns := o.Namespace
	if o.AllNamespaces {
		ns = "" // An empty string tells the client to query all namespaces.
	}

	// We need a REST client that can negotiate for Table output.
	restClient, err := NewRestClient(*o.RESTConfig, gvr.GroupVersion())
	if err != nil {
		return err
	}

	continueToken := o.ContinueToken
	isFirstRequest := true

	for {
		listOptions := metav1.ListOptions{
			Limit:         o.Limit,
			Continue:      continueToken,
			LabelSelector: o.Selector,
		}

		table := &metav1.Table{}
		err := restClient.Get().
			Namespace(ns).
			Resource(gvr.Resource).
			VersionedParams(&listOptions, scheme.ParameterCodec).
			Do(context.Background()).
			Into(table)
		if err != nil {
			return err
		}

		// If it's the first page and there are no items, just say so and exit.
		if isFirstRequest && len(table.Rows) == 0 {
			fmt.Fprintln(o.Out, "No resources found.")
			return nil
		}

		// Directly create a table printer to ensure correct output.
		printer := printers.NewTablePrinter(printers.PrintOptions{})
		if err := printer.PrintObj(table, o.Out); err != nil {
			return err
		}

		isFirstRequest = false
		continueToken = table.Continue

		// If there's no token, we've reached the end of the list.
		if continueToken == "" {
			if o.Interactive {
				fmt.Fprintln(o.Out, "\n--- End of list ---")
			}
			return nil
		}

		// Handle pagination flow.
		if o.Interactive {
			fmt.Fprintf(o.Out, "\n--- [n] next page, [q] quit: ")
			reader := bufio.NewReader(os.Stdin)
			char, _, err := reader.ReadRune()
			if err != nil {
				return err
			}
			fmt.Println() // Newline for clean formatting after user input.
			if char != 'n' {
				return nil // Quit on any key other than 'n'.
			}
		} else {
			// In non-interactive mode, print the token and exit.
			fmt.Fprintf(o.Out, "\nContinue Token: %s\n", continueToken)
			return nil
		}
	}
}

// NewRestClient creates a REST client configured to request Table-formatted server-side printing.
func NewRestClient(config rest.Config, gv schema.GroupVersion) (rest.Interface, error) {
	config.GroupVersion = &gv
	config.APIPath = "/apis"
	if gv.Group == "" {
		config.APIPath = "/api"
	}
	config.AcceptContentTypes = "application/json;as=Table;v=v1;g=meta.k8s.io,application/json"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()

	return rest.RESTClientFor(&config)
}

// GetResourceGVR finds the GroupVersionResource for a given short resource name.
func (o *HeadOptions) GetResourceGVR() (schema.GroupVersionResource, error) {
	resourceArg := strings.ToLower(o.Resource)

	// Create a partial GVR from the user's argument. We don't know the version,
	// so we leave it empty. The RESTMapper will find the best match.
	// This approach handles "pods", "deployments", and "deployments.apps" style arguments.
	gvrToFind := schema.GroupVersionResource{}
	parts := strings.Split(resourceArg, ".")
	if len(parts) == 2 {
		gvrToFind = schema.GroupVersionResource{Group: parts[1], Resource: parts[0]}
	} else {
		gvrToFind = schema.GroupVersionResource{Resource: resourceArg}
	}

	gvr, err := o.Mapper.ResourceFor(gvrToFind)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("the server doesn't have a resource type %q", o.Resource)
	}

	return gvr, nil
}