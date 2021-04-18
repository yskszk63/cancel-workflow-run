const GITHUB_WEB_ENDPOINT = "https://github.com";
const GITHUB_API_ENDPOINT = "https://api.github.com";

const $form = document.querySelector('form[name=appnew]');
const $herokuEndpoint = document.querySelector('[name=heroku-endpoint]');
const $pre = document.querySelector('#heroku-conf-code');
const ghapp = sessionStorage.getItem("ghapp");
const herokuAppname = sessionStorage.getItem("heroku-appname");

const copyButton = new ClipboardJS('#copy-heroku-cli', {
    text(trigger) {
        return trigger.parentElement.querySelector('code').textContent;
    }
});

$form.addEventListener('submit', evt => {
    const url = new URL(window.location);
    url.search = "";

    // TODO CSRF token
    const manifest = {
      "name": "Cancel workflow run.",
      "url": url,
      "hook_attributes": {
        "url": `https://${$herokuEndpoint.value}.herokuapp.com/webhook`,
      },
      "redirect_url": url,
      "public": false,
      "default_permissions": {
        "actions": "write",
        "pull_requests": "write",
        "metadata": "read",
      },
      "default_events": [
        "workflow_run"
      ]
    };
    sessionStorage.setItem('heroku-appname', $herokuEndpoint.value);
    evt.target.querySelector('[name=manifest]').value = JSON.stringify(manifest);
    evt.target.action = new URL('/settings/apps/new?state=abc123', GITHUB_WEB_ENDPOINT);
});

if (!!herokuAppname) {
    $herokuEndpoint.value = herokuAppname;
}

if (!!ghapp && !!herokuAppname) {
    const {app_id, webhook_secret, secret} = JSON.parse(ghapp);
    $pre.textContent = `heroku config:set -a ${herokuAppname} -e APP_ID=${app_id} -e WEBHOOK_SECRET=${webhook_secret} -e SECRET=${JSON.stringify(secret)}`;
}

if (!!window.location.search) {
    const params = new URLSearchParams(window.location.search.slice(1));
    const code = params.get('code');
    if (code !== null) {
        (async () => {
            // TODO CSRF token
            const url = new URL(`/app-manifests/${code}/conversions`, GITHUB_API_ENDPOINT);
            const headers = {
                'Accept': 'application/vnd.github.v3+json',
            };
            const response = await fetch(url, {
                headers,
                method: 'POST',
                mode: 'cors',
            });
            if (!response.ok) {
                const text = await response.text();
                throw Error(`${response.status} ${text}`);
            }
            const {id: app_id, webhook_secret, pem: secret} = await response.json();
            sessionStorage.setItem('ghapp', JSON.stringify({
                app_id,
                webhook_secret,
                secret,
            }));

            const newurl = new URL(window.location);
            newurl.search = "";
            window.location.assign(newurl);
        })();
    }
}
