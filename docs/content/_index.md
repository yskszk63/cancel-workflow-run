+++
title = "Cancel Workflow Run"
description = "Cancel Workflow Run"
+++

# Installation
## 1. Setup Webhook backend (Heroku)

[![Deploy](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy?template=https://github.com/yskszk63/cancel-workflow-run)

## 2. Create GitHub Apps

<form name="appnew" action="" method="post">
  <div>
    https://<input type="text" name="heroku-endpoint" value="" placeholder="heroku app name here." required/>.herokuapp.com/webhook
  </div>
  <input type="hidden" name="manifest" />
  <button type="submit">Create GitHub Apps</button>
</form>

## 3. Configure Heroku App

<pre id="heroku-conf">
  <div id="copy-heroku-cli" type="button">
    <span class="material-icons">content_copy</span>
  </div>
  <code id="heroku-conf-code"></code>
</pre>
