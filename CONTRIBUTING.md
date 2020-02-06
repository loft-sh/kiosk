# Contributing

## Guidelines

Guidelines for contributing.

### How can I get involved?

There are a number of areas where contributions can be accepted:

- Write Golang code for the Api Server Extension, Resource Controller or other components
- Write examples
- Review pull requests
- Test out new features or work-in-progress
- Get involved in design reviews and technical proof-of-concepts (PoCs)
- Help release and package kiosk including the helm chart, compose files, `kubectl` YAML, marketplaces and stores
- Manage, triage and research Issues and Pull Requests
- Engage with the growing community by providing technical support on GitHub
- Create docs, guides and write blogs

This is just a short list of ideas, if you have other ideas for contributing please make a suggestion.

### Setup kiosk for development

We recommend to develop kiosk directly inside a kubernetes cluster. The easiest way is to use the local kubernetes cluster provided by Docker Desktop for Windows/Mac or minikube for Linux.

After you setup the kubernetes cluster, you have to install [DevSpace](https://github.com/devspace-cloud/devspace#1-install-devspace), which will take care of the hot-reloading.

Make sure you have cert-manager installed in your cluster via:

```
# Install cert manager with helm 3
kubectl create namespace cert-manager
helm install cert-manager cert-manager --repo https://charts.jetstack.io --version v0.12.0 --namespace cert-manager
```

Then start working on kiosk:

```
# This will build and deploy kiosk into the kiosk namespace

# To develop the manager
devspace run dev-manager

# To develop the extension apiserver
devspace run dev-apiserver
```

As soon as the terminal pops up start kiosk via:

```
# Start the manager
go run -mod vendor cmd/manager/main.go

# OR start the apiserver, depending on which devspace run ... you executed
go run -mod vendor cmd/apiserver/main.go
```

You can then change files locally and just restart kiosk in the terminal. If you get an error during redeploying that the helm chart couldn't be deployed because the apiservice `tenancy.kiosk.sh` is not reachable, run:

```
# delete the apiservice and webhook
devspace run clean

# redeploy kiosk again
devspace run dev-...
```

### I want to contribute on GitHub

**Please do not raise a proposal after doing the work - this is counter to the spirit of the project. It is hard to be objective about something which has already been done**

What makes a good proposal?

- Brief summary including motivation/context
- Any design changes
- Pros + Cons
- Effort required up front
- Effort required for CI/CD, release, ongoing maintenance
- Migration strategy / backwards-compatibility
- Mock-up screenshots or examples of how the CLI would work
- Clear examples of how to reproduce any issue the proposal is addressing

Once your proposal receives a `design/approved` label you may go ahead and start work on your Pull Request.

If you are proposing a new tool or service please do due diligence. Does this tool already exist in a 3rd party project or library? Can we reuse it?

Every effort will be made to work with contributors who do not follow the process. Your PR may be closed or marked as `invalid` if it is left inactive, or the proposal cannot move into a `design/approved` status.

## License

This project is licensed under the Apache 2.0 License.

### Copyright notice

It is important to state that you retain copyright for your contributions, but agree to license them for usage by the project and author(s) under the Apache 2.0 license. Git retains history of authorship, but we use a catch-all statement rather than individual names.
