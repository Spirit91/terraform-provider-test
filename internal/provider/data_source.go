// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ datasource.DataSource = (*testDataSource)(nil)
)

func NewTestDataSource() datasource.DataSource {
	return &testDataSource{}
}

type testDataSource struct{}

func (n *testDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName
}

func (n *testDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Executes a local command, just like `local-exec`, and returns its output.",
		Attributes: map[string]schema.Attribute{
			"command": schema.StringAttribute{
				Description: "The command to execute.",
				Required:    true,
			},
			"working_dir": schema.StringAttribute{
				Description: "Working directory of the program. Defaults to the current directory.",
				Optional:    true,
			},
			"output": schema.StringAttribute{
				Description: "The standard output of the executed command.",
				Computed:    true,
			},
			"error": schema.StringAttribute{
				Description: "The standard error output of the executed command.",
				Computed:    true,
			},
			"id": schema.StringAttribute{
				Description: "The ID of the data source, always set to `-`.",
				Computed:    true,
			},
		},
	}
}

func (n *testDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config testDataSourceModel

	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Récupération des valeurs
	command := config.Command.ValueString()
	workingDir := config.WorkingDir.ValueString()

	if command == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("command"),
			"Missing Command",
			"The command cannot be empty. Please specify a valid shell command.",
		)
		return
	}

	// Vérification de l'exécutable
	_, err := exec.LookPath(strings.Fields(command)[0])
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("command"),
			"Command Not Found",
			fmt.Sprintf("The command '%s' was not found. Ensure it's installed and accessible.", command),
		)
		return
	}

	// Préparation de l'exécution
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	if workingDir != "" {
		cmd.Dir = workingDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	tflog.Trace(ctx, "Executing command", map[string]interface{}{"command": command})

	err = cmd.Run()
	outputStr := stdout.String()
	errorStr := stderr.String()

	tflog.Trace(ctx, "Executed command", map[string]interface{}{
		"command": command,
		"output":  outputStr,
		"error":   errorStr,
	})

	// Si la commande a échoué
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("command"),
			"Command Execution Failed",
			fmt.Sprintf("Command execution failed.\n\nCommand: %s\nError: %s\nStderr: %s", command, err, errorStr),
		)
		return
	}

	// Mettre à jour l'état
	config.Output = types.StringValue(outputStr)
	config.Error = types.StringValue(errorStr)
	config.ID = types.StringValue("-")

	diags = resp.State.Set(ctx, &config)
	resp.Diagnostics.Append(diags...)
}

type testDataSourceModel struct {
	Command    types.String `tfsdk:"command"`
	WorkingDir types.String `tfsdk:"working_dir"`
	Output     types.String `tfsdk:"output"`
	Error      types.String `tfsdk:"error"`
	ID         types.String `tfsdk:"id"`
}
