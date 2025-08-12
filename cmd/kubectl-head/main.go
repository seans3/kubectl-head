package main

import (
	"fmt"
	"os"

	"github.com/seans3/head/pkg/head"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func main() {
	streams := genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
	cmd := NewCmdHead(streams)

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// NewCmdHead creates a new cobra command that can be used to run the head logic.
func NewCmdHead(streams genericclioptions.IOStreams) *cobra.Command {
	o := head.NewHeadOptions(streams)

	cmd := &cobra.Command{
		Use:   "head [type]",
		Short: "Efficiently head at the first N resources from the API server",
		Long: `The "head" command allows you to retrieve just the first N items of a resource list,
avoiding the high memory and network usage of "kubectl get" on clusters with many resources.
It supports pagination through an interactive mode or by manually passing a continue token.`,
		Example: `
  # Head at the first 10 pods in the current namespace
  kubectl head pods

  # Head at the first 5 deployments in wide format
  kubectl head deployments --limit 5 -o wide

  # Interactively page through all services, 20 at a time
  kubectl head services --limit 20 -i

  # Get the second page of pods, using a token from a previous run
  kubectl head pods --limit 10 --continue "eyJhbGciOi..."
`,
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("you must specify the type of resource to head")
			}
			if len(args) > 1 {
				return fmt.Errorf("only one resource type is allowed")
			}
			
			if err := o.Complete(args[0]); err != nil {
				return err
			}
			if err := o.Validate(); err != nil {
				return err
			}
			if err := o.Run(); err != nil {
				return err
			}
			return nil
		},
	}

	// Add our custom flags.
	cmd.Flags().Int64Var(&o.Limit, "limit", head.DefaultHeadLimit, "Number of items to return per page.")
	cmd.Flags().StringVar(&o.ContinueToken, "continue", "", "A token used to retrieve the next page of results. If not provided, the first page is returned.")
	cmd.Flags().BoolVarP(&o.Interactive, "interactive", "i", false, "Enable interactive mode to page through results.")
	cmd.Flags().StringVarP(&o.Selector, "selector", "l", "", "Selector (label query) to filter on. Supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)")
	cmd.Flags().BoolVarP(&o.AllNamespaces, "all-namespaces", "A", false, "If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")

	// Add standard kubectl flags.
	o.ConfigFlags.AddFlags(cmd.Flags())
	o.PrintFlags.AddFlags(cmd)

	return cmd
}
