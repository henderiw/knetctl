/*
Copyright 2024 Nokia.

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

package clab2kuidcmd

import (
	"context"
	"fmt"
	"os"

	//docs "github.com/pkgserver-dev/pkgserver/internal/docs/generated/initdocs"

	"github.com/kubenet-dev/knetctl/pkg/clab"
	infrav1alpha1 "github.com/kuidio/kuid/apis/backend/infra/v1alpha1"
	"github.com/pkgserver-dev/pkgserver/pkg/client"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/yaml"
)

// NewRunner returns a command runner.
func NewRunner(ctx context.Context, version string, cfg *genericclioptions.ConfigFlags) *Runner {
	r := &Runner{}
	cmd := &cobra.Command{
		Use:  "clab2kuid CLABFILE DIR [flags]",
		Args: cobra.ExactArgs(2),
		//Short:   docs.InitShort,
		//Long:    docs.InitShort + "\n" + docs.InitLong,
		//Example: docs.InitExamples,
		PreRunE: r.preRunE,
		RunE:    r.runE,
	}

	r.Command = cmd
	r.cfg = cfg

	cmd.Flags().StringVar(&r.region, "region", "region1", "Region this topology belongs to")
	cmd.Flags().StringVar(&r.site, "site", "site1", "Site this topology belongs to")

	return r
}

func NewCommand(ctx context.Context, version string, kubeflags *genericclioptions.ConfigFlags) *cobra.Command {
	return NewRunner(ctx, version, kubeflags).Command
}

type Runner struct {
	Command *cobra.Command
	cfg     *genericclioptions.ConfigFlags
	client  client.Client
	region  string
	site    string
}

func (r *Runner) preRunE(_ *cobra.Command, _ []string) error {
	client, err := client.CreateClientWithFlags(r.cfg)
	if err != nil {
		return err
	}
	r.client = client
	return nil
}

func (r *Runner) runE(c *cobra.Command, args []string) error {
	ctx := c.Context()
	//log := log.FromContext(ctx)
	//log.Info("create packagerevision", "src", args[0], "dst", args[1])

	b, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}

	clab, err := clab.NewClabKuid(
		&infrav1alpha1.SiteID{
			Region: r.region,
			Site:   r.site},
		string(b))
	if err != nil {
		return err
	}

	for _, n := range clab.GetNodes(ctx) {
		fmt.Println("---")
		b, err := yaml.Marshal(n)
		if err != nil {
			return err
		}
		fmt.Println(string(b))

	}

	for _, l := range clab.GetLinks(ctx) {
		fmt.Println("---")
		b, err := yaml.Marshal(l)
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	}

	return nil
}
