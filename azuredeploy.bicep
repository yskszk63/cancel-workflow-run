// https://aka.ms/bicepdemo#eJzVVm1vIjcQ/s6v8IdIC1LYTXK9qE0VKVxuL6FpIAVyaXWKVmY9gI/Fdm1vyIvob+94l12Wy15K+61RBMYzfmbseZ6xg4DMrFXmJAjGPAbFYCH956MjfwljP5Ya/CUXTC6NL8AGjWDjzmRs/AWPtTRyYtF3EXyl7a8qoM+phvyzPUlFbLkUJihHbS4mmhqr09g6F2rasWTQaJwxMLHmynk1vQtuL9Mx6Shl3AfhzCf9zEYTn4xTS3oAzJCEWtBeq6GopgtCleKMIDgXU3JKPK9xZiDGMM3WN/gfqIHjHwgIF5yRargbzR8QlVzB0y4x8aRmUs4jDKTB7hS8Gu0uX06G2fJdAtYEajxQTRIZU7cWpzQYmeoYLrRMVbPlF6bMT2mY8Ee3MKYihmQ50V5mMFTQBTiDoXsvudtq7yUV/M8Uhlm45rfInLVW+Wo8+2I5Ds2/B+DC2ALBjf8bgqXThIs5gsRS4K6b3owzBqLtZk+8/Vdnw9k+8QKl5QNnoE1wXXIaSxMYbsEEuGy9vVajIVOrsDTFfss6rCcKuwGbqijVSaVShXj2XtbOKz8TCnIoC5SpjCoezCBJJNa1SBZrQ7xNZkMrNZ1CYPLvThzLVFhzdnRwdNg+cP8ehntpEOKCnKwriz8LIpyUI5yco8RPiLcG/Xzk4ZyZpycZQAGBZioY1Sz6dTB0HqvGqpLfOJHjHTIMnN8Q9AM2m7p0y2h7L3nOq4DBhKaJ9TIrlkmBthxMkR3BOuvKr/z3IE2cy5dyklQc3B/F810C62s+RdZse7q/slRKaotqzMrkupy35XhfB3oNdiZZHehFOHpr/YI+dqbQFdgLpHAAh1tmeFTSALsE6oiK8LXR682rclzM5jP5JwMFGLAvNjkb2si9q1V2AqtWuSsMn86wrHgwSgrIOXj4vn3w/hUHC3n/AwtRCu6IUMZlUZEMFWWvPPQarDPKiVjHC2yuCc/Bo9GTgg30NnFdr6puqRS928iP7cPXYip0/vY+igsPvfcx7fTRq03ThTqXYsKnGwrjkiFYiz1ji0NVAq9V0nGsxIx/kWOzVlyVXw80SdGtaIQfcyWFginJsVQ3WloZy+Q04/rPa432EPoUG56hvouCzbGw4IXoDAk3bmiaGw88qp/aB8fuqFr+HG1fDu79LHqrTGf11kY+3fbOR91+bxjd9QdX4SAa3PZG3euwZjdenGJTqehwR+Dw91HYG+Iw+hwO3Hcd9l/vdsTt3Nx0Ee3icjSMcDAa3F6HvVHHhboK/6iBxqsMND43oLy/uqzpbR5Q/LWU3LVTqKaVn3KhrJaTHr6hFuiXEQ8Lslvid+GHYXcUugOOPg3619FN5/yqc1F70kUPnHI7S8fZG+/JzM3z/PhdkL8d2kup5xPsO22dikBDAviuMvg4XIpEUha4l4uxgaLxHLnpP3O14/n+loK78DNKoz4EZGLKXwD/T4ojY6Lux5rcs0frzsW77PevomF4Pti+StZY24/R3UC/C/YdkPvKxYFcxsscsOEhGSFvrn8DGrm43A==
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
    dependsOn: [
      sa
    ]
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
