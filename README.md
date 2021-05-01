# Cancel workflow run GitHub Apps

[![Go Report Card](https://goreportcard.com/badge/github.com/yskszk63/cancel-workflow-run)](https://goreportcard.com/report/github.com/yskszk63/cancel-workflow-run)
[![codecov](https://codecov.io/gh/yskszk63/cancel-workflow-run/branch/main/graph/badge.svg?token=RFUNJ8CH9R)](https://codecov.io/gh/yskszk63/cancel-workflow-run)
[![](https://tokei.rs/b1/github/yskszk63/cancel-workflow-run)](https://github.com/XAMPPRocky/tokei)
[![deploy](https://github.com/yskszk63/cancel-workflow-run/actions/workflows/deploy.yml/badge.svg)](https://github.com/yskszk63/cancel-workflow-run/actions/workflows/deploy.yml)

Cancel workflow run BOT.
Cancel workflow run If Pull request contains new workflow, and which trigger pull request.

The main purpose is to repel miners.

## Install

Sorry, Not publish this GitHub Apps.
Because I don't have the money to get the backend working.

## GitHub Apps Setup flow (For Self hosted)

### Prerequisites

- GitHub Account.
- Azure Subscription.
    + Needs Contributor role

### 1. Click Deploy to Azure button

[![Deploy to Azure](https://aka.ms/deploytoazurebutton)](https://portal.azure.com/#create/Microsoft.Template/uri/https%3A%2F%2Fraw.githubusercontent.com%2Fyskszk63%2Fcancel-workflow-run%2Fmain%2Fazuredeploy.json)

### 2. Fill below And click Create

- Resource group ... Please specify the deployment destination of resources.
- Region ... Please specify the deployment region of resources.
- AppId ... NO NEED TO FILL. (fill after GitHub Apps created.)
- Webhook\_secret ... NO NEED TO FILL. (fill after GitHub Apps created.)
- Secret ... NO NEED TO FILL. (fill after GitHub Apps created.)

![setup-1](assets/setup-1.png)
![setup-2](assets/setup-2.png)

### 3. After deployed. Open `setup_url`

![setup-3](assets/setup-3.png)

### 4. Create GitHub Apps.

After `setup_url` opened. GitHub App Creation Site will be displayed.
Fill GitHub App name, And click Create GitHub App Button.

![setup-4](assets/setup-4.png)

### 5. Apply GitHub App Parameters.

After click Create GitHub App Button. Azure template deploy screen displan again.
Select Resource Group which fill in `2. Fill below And click Create`.

![setup-5](assets/setup-5.png)

### 6. Done.

You can now install it. 

## Using resources.

![archtecture](assets/architecture.png)

- Azure Functions
    + Linux consumption plan
    + (Backend language is go)
- Azure Storage
    + Azure Functions Requirements
- Azure App Insights
    + For Azure Functions monitoring
- Event Grid
    + Kick workflow cancel job for GitHub App webhook hooked Asynchronously.

## Details

### Trigger `workflow_run` flow

![process](assets/process_dia.png)

### Setup GitHub App flow

![setup1](assets/setupflow_dia_1.png)
![setup2](assets/setupflow_dia_2.png)

It is drawn in [yskszk63/seqdia](https://github.com/yskszk63/seqdia).

## LICENSE

[MIT](LICENSE)
