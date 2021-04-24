+++
title = "Cancel Workflow Run"
description = "Cancel Workflow Run"
+++

# prerequisite

- GitHub Account
- Azure Subscription
- az cli
- bash
- findutils (xargs)

# Installation
## 1. Setup Webhook backend (Azure Functions)

[![Deploy to Azure](https://aka.ms/deploytoazurebutton)](https://portal.azure.com/#create/Microsoft.Template/uri/https%3A%2F%2Fraw.githubusercontent.com%2Fyskszk63%2Fcancel-workflow-run%2Fmain%2Fazuredeploy.json)

## 2. Create GitHub Apps

<form name="appnew" action="" method="post">
  <div>
    https://<input type="text" name="af-endpoint" value="" placeholder="azure functions name here." required/>.azurewebsites.net/api/webhook
  </div>
  <input type="hidden" name="manifest" />
  <button type="submit">Create GitHub Apps</button>
</form>

## 3. Configure Azure Functions

<pre id="conf">
  <div id="copy-cli" type="button">
    <span class="material-icons">content_copy</span>
  </div>
  <code id="conf-code"></code>
</pre>
