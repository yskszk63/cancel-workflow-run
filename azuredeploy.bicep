// https://aka.ms/bicepdemo#eJzVVm1v2zYQ/u5fwQ8BZAOxlKRrsHkIEDdVEy+LndlOs6EIBFo8O6xlkiOpOC/wfvuOkiXLjZp5+zbDkCje8bnj3XNHBgG5t1aZThBMeAyKwUL6z0dH/hImfiw1+EsumFwaX4ANGsFGncnY+Asea2nk1KLuIvhK219VQJ9TDfmzPU1FbLkUJihHbS6mmhqr09g6FWrasWTQaJwyMLHmymk1vXNuL9IJ6Spl3INw5pNBJqOJTyapJX0AZkhCLWiv1VBU0wWhSnFGEJyLGTkhntc4NRCjmWbrG/wP1MDxDwSEM85I1dy15g+ISi7haRebGKl7KecRGtJgdzJetXabLyejbPkuBmsMNR6oJomMqVuLUxqMTHUM51qmqtnyC1GmpzRM+aNbGFMRQ7Kcai8TGCroApzA0L2XXG2195IK/mcKo8xc81tkzlqrfDXGvliOQ/PvAbgwtkBw4/+GYOks4WKOILEUuOumd88ZA9F2sx1v/1VsONsnXqC0fOAMtAmuSk5jagLDLZgAl62312o0ZGoVpqbYb5mH9UQhN2BTFaU6qWSqKJ69l7Xyys8KBTmUGcqqjCoeuJ3QJIlm3N6nkwi1McmF55go4m3cHFmp6QwCk7+7cSxTYc3p0cHRYfvA/T20/dIgxFnsrNOMnwUrOuUIJ+dY7x3irUE/H3k4Z+ZpJwMoIFBMBaOaRb8OR05j1VhV/JskcrKDh4HTG4F+wM5T525pbe8l93kVMJjSNLFeJsWcKdCWgym8I5h0XfnKv4dp4lS+lJOkouB+GGm5BDbQfIaB39Z0vzJvSmpMS54z1/K8LcW7OtArsPeS1YGeh+O31i/oY3cGPYGNQQoHcLglhkclDbALoI61CF9rvV68KsfFbD6TPxkoQIMDsfHZ0EauXc2y42g1yz1h+Owe04qBUVJAzsHD9+2D9684WNT6P7AQ68KFCGu6TCqSoVLmKw+1hmuPciLW8QI7bcJz8Gj8pGADvU1c17iqWyo7gNvIj+3D18VUFP3b+yhOP9TeR7fTR6/WTWfqTIopn20ojEtGYC02kC0OVQm8rpKuYyV6/IucmHXFVfn1QJMU1Yqu+DGvpFAwJTmm6lpLK2OZnGRc/3ldo32EPsHuZ6jvrGCnLCR4OjpBwo0bmuZGA0P1U/vg2IWq5c9R9uXgzs+st0p3Vm9t5NNN/2zcG/RH0e1geBkOo+FNf9y7Cmt248UpNpVKHe4IHP4+DvsjHEafw6F712H/9W5H3O71dQ/Rzi/GowgH4+HNVdgfd52py/CPGmg810Dj3QPKw6zHmt7mNsVfl5I7g4qqaeVRLiqr5UoPL1QL1MuIhwnZzfHb8MOoNw5dgKNPw8FVdN09u+ye10a66IH5kZRd+J7M3DzPj98F+UWivZR6PsW+09apCDQkgJcsgzfFpUgkZYG7xhgbKBrPkZv+M1c7xve3FNzpn1Ea60NAVkz5deD/SXFkTNT7WON7doPdOXkXg8FlNArPhttHyRpr+2a6G+h3wb4Dclc5OJDLeJgDNjwkI+TN9W/uDL5B
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
output setup_url string = 'https://${appname}.azurewebsites.net/api/install_github_app'

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
