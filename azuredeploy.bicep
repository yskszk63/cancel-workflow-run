// https://aka.ms/bicepdemo#eJzVVm1vIjcQ/s6v8IdIC1LYTXK9qE0VKVxuL6FpIAVyaXWKVmY9gI/Fdm1vyIvob+94l12Wy15K+60oCsYzfmbseZ6xg4DMrFXmJAjGPAbFYCH956MjfwljP5Ya/CUXTC6NL8AGjWDjzmRs/AWPtTRyYtF3EXyl7a8qoM+phvx/e5KK2HIpTFCO2lxMNDVWp7F1LtS0Y8mg0ThjYGLNlfNqehfcXqZj0lHKuH+EM5/0MxtNfDJOLekBMEMSakF7rYaimi4IVYozguBcTMkp8bzGmYEYwzRb3+B/oAaOfyAgXHBGquFuNH9AVHIFT7vExJOaSTmPMJAGu1PwarS7fDkZZst3CVgTqPFANUlkTN1anNJgZKpjuNAyVc2WX5gyP6Vhwh/dwpiKGJLlRHuZwVBBF+AMhu695G6rvZdU8D9TGGbhmt8ic9Za5avx7IvlODT/HoALYwsEN/5vCJZOEy7mCBJLgbtuejPOGIi2mz3x9l+dDWf7xAuUlg+cgTbBdclpLE1guAUT4LL19lqNhkytwtIU+y3rsJ4o7AZsqqJUJ5VKFeLZe1k7r/xMKMihLFCmMqp4MIMkkVjXIlmsDfE2mQ2t1HQKgcm/O3EsU2HN2dHB0WH7wP15GO6lQYgLcrKuLP4siHBSjnByjhI/Id4a9PORh3Nmnp5kAAUEmqlgVLPo18HQeawaq0p+40SOd8gwcH5D0A/YbOrSLaPtveQ5rwIGE5om1susWCYF2nIwRXYE66wrv/LfgzRxLl/KSVJxcB+K57sE1td8iqzZ9nSfslRKaotqzMrkupy35XhfB3oNdiZZHehFOHpr/YI+dqbQFdgLpHAAh1tmeFTSALsE6oiK8LXR682rclzM5jPbVXQCqlaxKwyfzrBsuHElBeQcO3zfPnj/imOFfP+BZUh1dwQo07JoWOyKclceeg3WGeVEq6s7Ns+E5+DR6EnBBnp7S64XVbdUitpt5Mf24WuxFDp+ex/FhYbe+5h2+ujVpulCnUsx4dMNRXHJEKzFnrDFkSpB1yroONZhxr/IsVkrqsqfB5qk6FY0uo+5UkLBlORYqhstrYxlcppx+ee1BnsIfYoNzVDfRcHmV1jwwnOGhBs3NM2NBx7VT+2DY3dULX+Oti8H934WvVWms3prI59ue+ejbr83jO76g6twEA1ue6PudVizGy9OsWlUdLYjcPj7KOwNcRh9Dgfuuw77r3c74nZubrqIdnE5GkY4GA1ur8PeqONCXYV/1EDjVQUanxNQ3k9d1vQ2DyT+WkruWilU08pPuVBWy0kP30gL9MuIhwXZLfG78MOwOwrdAUefBv3r6KZzftW5qD3posdNuZ2l4+wN92Tm5nl+/C7I3wbtpdTzCfaVtk5FoCEBfDcZfPwtRSIpC9zLxNhA0XiO3PSfudrxfH9LwV3oGaVRHwIyMeU3/P+T4siYqPuxJvfsUbpz8S77/atoGJ4Ptq+KNdb2Y3M30O+CfQfkvrH5hVzGyxqw4SEZIW+ufwPu57F4
// https://bicepdemo.z22.web.core.windows.net/
// https://docs.microsoft.com/ja-jp/azure/azure-functions/functions-infrastructure-as-code

@description('GitHub Apps App id. Optional. but Needs later')
param appid string = ''
@secure()
@description('Base64 encoded GitHub Apps Private Key. Optional. but Needs later')
param webhook_secret string = ''
@secure()
@description('GitHub Apps Webhook Secret. Optional. but Needs later')
param secret string = ''

var location = resourceGroup().location
var prefix = 'cancelwfr'
var saname = 'sa${prefix}${uniqueString(resourceGroup().id)}'
var appname = 'apps${prefix}${uniqueString(resourceGroup().id)}'
var instname = 'inst${prefix}${uniqueString(resourceGroup().id)}'
var insttaglink = concat('hidden-link:', resourceGroup().id, '/providers/Microsoft.Web/sites/', appname)

output appname string = appname
output setup_url string = 'https://${appname}.azurewebsites.net/api/hello'

resource sa 'Microsoft.Storage/storageAccounts@2021-01-01' = {
  name: saname
  location: location
  kind: 'StorageV2'
  sku: {
    name: 'Standard_LRS'
  }
}

resource blob 'Microsoft.Storage/storageAccounts/blobServices@2021-01-01' = {
    name: '${saname}/default'
    properties: {
      cors: {
        corsRules: [
          {
            allowedOrigins: [
              'https://portal.azure.com'
            ]
            allowedMethods: [
              'GET'
            ]
            maxAgeInSeconds: 1
            exposedHeaders: []
            allowedHeaders: []
          }
        ]
      }
    }
}

resource inst 'Microsoft.Insights/components@2015-05-01' = {
  name: instname
  location: location
  kind: 'web'
  tags: {
    '${insttaglink}': 'Resource'
  }
  properties: {
    Application_Type: 'web'
  }
}

resource apps 'Microsoft.Web/sites@2018-11-01' = {
  name: appname
  location: location
  kind: 'functionapp,linux'
  properties: {
    siteConfig: {
      appSettings: [
        {
          name: 'AzureWebJobsStorage'
          value: concat('DefaultEndpointsProtocol=https;AccountName=', sa.name, ';AccountKey=', listKeys(sa.name, '2019-06-01').keys[0].value)
        }
        {
          name: 'FUNCTIONS_WORKER_RUNTIME'
          value: 'custom'
        }
        {
          name: 'FUNCTIONS_EXTENSION_VERSION'
          value: '~3'
        }
        {
          name: 'APPINSIGHTS_INSTRUMENTATIONKEY'
          value: reference(resourceId('microsoft.insights/components/', instname), '2015-05-01').InstrumentationKey
        }
        {
          name: 'WEBSITE_RUN_FROM_PACKAGE'
          value: 'https://github.com/yskszk63/cancel-workflow-run/releases/download/latest/package.zip'
        }
        {
          name: 'QueueStorageConnectionString'
          value: concat('DefaultEndpointsProtocol=https;AccountName=', sa.name, ';AccountKey=', listKeys(sa.name, '2019-06-01').keys[0].value)
        }
        {
          name: 'APP_ID'
          value: appid
        }
        {
          name: 'WEBHOOK_SECRET'
          value: webhook_secret
        }
        {
          name: 'SECRET'
          value: secret
        }
      ]
    }
    reserved: true
  }
}
