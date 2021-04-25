// https://aka.ms/bicepdemo#eJzVVm1z4jYQ/u5foQ+ZMczEdpLrZVo6mQmX8yWUBlIgl3ZuMh5hLaDDSK4kh7wM/e1d2djAxZfSfquHwbJ29exqd5+VgoDMjEl1KwjGPIaUwUL6zycn/hLGfiwV+EsumFxqX4AJnGCjzmSs/QWPldRyYlB3EXyl3tc0oM+ZguLfm2QiNlwKHVQjj4uJotqoLDZWhWovlgwc55yBjhVPrVbDveTmKhuTdppq+0c480k/l9HEJ+PMkB4A0yShBpTbdFKq6ILQNOWMIDgXU3JGXNc51xCjmUbzG/wPVMPpDwSENc7ItrkbxR8QlXThaR+bGKmZlPMIDSkwexnftnZXLCfDfPk+BmsMOQ9UkUTG1K7FKQVaZiqGSyWztNH0S1GulyqY8Ee7MKYihmQ5UW4u0FTQBViBpgcvhdrq4CUT/M8Mhrm5xrfInDVXxWqMfbkch/rfA3ChTYlgx/8NwdBpwsUcQWIpcNcNd8YZA+HZ2ZZ7+Co2nB0SN0iVfOAMlA6uq5rG1ASaG9ABLltvr+k4MjMppqbcb5WH9YTjlBYwoMTdwA2NVHQKgS7e7TiWmTD6/OTo5Ng7sj8XUV4cQixMa50O/Cyz16pGODlHXraIuwb9fOLinJ5nrRyghEAxFYwqFv06GFqNlbPa8m+cyPEeHgZWbwjqATtEnbuVtYOXwudVwGBCs8S4uRRjm4IyHHTpHcHkqK2v4nuQJVblSzVJthTsQ5NELoH1FZ9iqnc17eOWzSmVyiCF8iZkW5O7o3hfB3oNZiZZHehlOHpr/YI+tqfQEUhgKSzA8Y4YHlOpgV0BtdWF8LXW68WralzOFjO7WbRVv53FjtB8OsO04cZTKaCoseP33tH7VzVWcu4fqgx7nA0BcqtKGiZ7i24rF7UGa4+KQqvLO3a8hBfg0egphQ307pZsA9neUsVEu5EfvePXZCnJ9/Y+ylMItQ/R7ezRrXXTmrqQYsKnmxLFJUMwBqm+UyPbBbpmQdtWHXr8ixzrNaO26+eBJhmqld3pY8GUULBUckzVjZJGxjI5y2v55zUHewh9hl1IU99awY5VSvCUsoKEazvUjY0Ghuon7+jUhqrpz1H25ejez603K3dWb23k023vYtTp94bRXX/QDQfR4LY36lyHNbtx4wybxhbP9gQOfx+FvSEOo8/hwL7rsP96tydu++amg2iXV6NhhIPR4PY67I3a1lQ3/KMGGs8XUHgHgOpQ6bCGu7nV8NdUsmdByZpmEeWSWU1LPbzYLFAvLzxMyH6O34Ufhp1RaAMcfRr0r6Ob9kW3fVkb6bLHTbmZZeP84vWk5/p5fvouKA50bynVfIJ9xVOZCBQkgJcdjTe2pUgkZYG9TmgTpDSeY236zzzdM76/ZWBP4bykkR8CcjIVx/L/s8SxYqLOxxrf85vk3sm76ve70TC8GOweFWus3RvifqDfBfsOyL2z+cJaxsMasOFhMULRXP8GTCCVkw==
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
