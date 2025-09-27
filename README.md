# k3s-mgmt-tool

A small CLI to describe and manage a Kubernetes/K3s environment from a single declarative file (Buildfile.yaml). Define your Helm repositories, releases, plain Kubernetes manifests, and startup scripts in one place, then let the tool help you generate per-chart values.yaml templates, verify your setup, sync with your cluster or your local Helm state, and apply changes.

## Features
- Single Buildfile.yaml to describe:
  - Helm repositories (name -> URL)
  - Helm releases (release name -> chart reference)
  - Kubernetes manifest file paths
  - Startup shell scripts to run after apply
  - Optional Git repositories (name -> URL)
- Generate default values.yaml files for all charts into _reposetup/<chart>/values.yaml using Helm show values
- Verify and diff your Buildfile against the local Helm repo config and against installed releases in the cluster
- Sync helpers to add missing local Helm repos and/or add installed releases into the Buildfile
- TUI/terminal-editor helpers for editing the Buildfile

## Prerequisites
- Go 1.25+
- Helm 3.9+ installed and configured (needed for reposetup and some sync/verify functions)
- kubectl configured to point at your cluster (for verification/sync of releases)
- Optional: k3d/K3s if you use those locally

## Install / Build
- Local build:
  - Clone this repository
  - go build -o k3s-mgmt-tool
- Or install into GOPATH/bin:
  - go install nexus-sites.net/k3s-mgmt-tool@latest

## Quick start
1) Initialize a new Buildfile.yaml
   k3s-mgmt-tool init

2) Add Helm repositories and releases
   k3s-mgmt-tool addRepo traefik https://traefik.github.io/charts
   k3s-mgmt-tool addChart traefik traefik/traefik

3) Optionally add K8s manifests and startup scripts
   k3s-mgmt-tool addManifest k8s/base/namespace.yaml
   k3s-mgmt-tool addScript scripts/post-install.sh

4) Generate per-chart values.yaml templates under _reposetup
   k3s-mgmt-tool reposetup
   - Use --force to overwrite existing files, --dry-run to preview, and --helm-binary if helm is not on PATH

5) Inspect and verify
   k3s-mgmt-tool verifyFile

6) Sync helpers (optional)
   - Read from your environment and update Buildfile
   k3s-mgmt-tool syncFromCluster --add-missing-repos
   k3s-mgmt-tool syncFromCluster --add-missing-charts
   k3s-mgmt-tool syncFromCluster --all

7) Apply to cluster (see notes below)
   k3s-mgmt-tool syncToCluster

Note: syncToCluster integrates Helm and kubectl to: ensure Helm repos exist, ensure releases/charts are present (respecting values under _reposetup), apply manifests, and run startup scripts. Ensure your kubeconfig and Helm env are set correctly before running.

## Command reference (selected)
- init
  - Create a new empty Buildfile.yaml if missing
- editBuildfile
  - Open Buildfile.yaml in your terminal editor (VISUAL/EDITOR respected; falls back to nano)
- addRepo <name> <url>
- removeRepo <name>
- addChart <release> <chart>
- removeChart <release>
- addManifest <path>
- removeManifest <path>
- addScript <path>
- removeScript <path>
- addGitRepo <name> <url>
- removeGitRepo <name>
- verifyFile
  - Print Buildfile contents and compare with local Helm repos and cluster-installed releases
- reposetup [--force] [--dry-run] [--helm-binary=helm]
  - Generate _reposetup/<chart>/values.yaml for every chart in Buildfile.yaml
  - If chart is unqualified (no '/'), tries repo-qualified forms based on defined Helm repos
- syncFromCluster [--add-missing-repos] [--add-missing-charts] [--all]
  - Update Buildfile from current environment
- syncToCluster
  - Sync Buildfile to the cluster (ensure repos, install/upgrade releases, apply manifests, run scripts)

Global flags
- --config path  Use a custom config file (default $HOME/.k3s-mgmt-tool.yaml)
- -v, --verbose  Verbose output for supported commands

## Buildfile.yaml schema (high level)
- helm_repos: map[name]url
- helm_charts: map[release]chartRef
- manifests: list of file paths
- startup_scripts: list of script paths
- git_repos: map[name]url

Example:

  helm_repos:
    traefik: https://traefik.github.io/charts
  helm_charts:
    traefik: traefik/traefik
  manifests:
    - k8s/base/namespace.yaml
  startup_scripts:
    - scripts/post-install.sh
  git_repos:
    my-infra: git@github.com:me/infra.git

Values files
- reposetup writes default values to _reposetup/<chart>/values.yaml (creating directories as needed)
- If Helm values cannot be retrieved, a placeholder file is created for you to edit manually

## Repository layout
- Buildfile.yaml
- cmd/ ... Cobra commands implementing the CLI
- pkg/buildfile ... Buildfile model + helpers (diff with local Helm, diff with installed releases, etc.)
- pkg/util ... small utilities
- _reposetup/ ... generated values.yaml templates per chart

## License
This project is released under the MIT License. See LICENSE for details.

## Acknowledgements
- Built with Cobra and Viper
- Uses Helm v3 libraries under the hood for several operations
