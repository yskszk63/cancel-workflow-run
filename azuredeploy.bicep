// https://aka.ms/bicepdemo#eJydV21v4jgQ/s6vyIdKAakktHtb3XGqVLabUq5XqAjd3mlVRSYZgpdg52wH+iLut984b4Q27dKLEBh7/Mx4Xp5xbNuYKxXLrm1PqQ9xAEtuPR0fW2uYWj4XYK0pC/haWgyU3bC34gH3pbWkvuCSzxTKLu0fpP0jtslTIiD7bs8S5ivKmbTLUZuymSBSicRXWoTIts8DaDTOApC+oLGWapp9qi6TqdGLY6m/DBpYxihdI5FlTBNlDAECaUREgTBbjZgIsjRIHNPAQHDKQuPUMM3GmQQf1TRbL/C/EAknvxjAtPLAqKq7EXSFqMYVPO6jEz0153zhoSIBai/lVW132XbDTbfvo7BGUWNFhBFxn+i9OCVA8kT40Bc8iZstq1hK5WIBM/qgN/qE+RCtZ8JMFyRhZAl6QZKD50xsc/CcMPpPAm6qrvkSmQatTbYbfV9sx6H8OABlUhUIevz/EBQJI8oWCOJzhqdumnMaBMDaerZrHr7yDQ0ODdOOBV/RAIS0r8ucxtDYkiqQNm7Lj9dKFcEKmAoFDQp7IfyYtQ2eqBjjWzitDGY+UaxLUEnsJSKqhLuowIPnXHhjpdWGiZham5YqiamdbQ6pmidTD2VRbWENxtowtyd1FRckBFtmvz3f5wlT8uy4c3zU7uiPiZqfG4ah9XXzTMG/RWJ1yxFOLpAyuoaZg347NnFOLpJuClBA4DJhARGB9+fY1RKbxqZi3zTi0z0stLWcC2KF5FVnbqnt4DmzeWMHMCNJpMx0FcMeg1AUZGGdgXkjKv+y/+Mk0iLfy0mjIqAfEkV8DcFI0BCzcFdSP2XUYi4UVncaMc2a5o7gfR3oNag5D+pA+87kvf1L8tALYcCQWzjTAEc7y/AQcwnBJRCd+Ahfq71+eVOOi9lsJvsOIAZUOGJbmyVpZNLVKJd1VA21oyf7OGkrHlM/DWun3TmpCetOHabzNQm5ozCUybRk4nqtKahbEXvPAkysHSM22OiykEgfU6ti4pv5hq1BUZZbvc0qdGDMKVOTRw1j9nTCXORddBv0OkD9FCceYCVux02zjt3K3pzRHBIIHgRJEbGxqiQ2npdR/0msNUhNtDU9V10+YJKGcyxiLIOYM8gY5+hzu/P5FeMUzeEnnIMcqH2DTaD0B0ao0hc2JkqNc4sy2qlzIrbmiGbgXh6AHHqXpvRJjTqn6oP82j56TZ0Fwb9/jiIkKH2IZicPZq2ZWtU5ZzMabqOPW1xQmFHhDmNUkyNP3TSl0OI/+FTm/FplkxWJEhQr2ujXjDedPC3ljeCK+zw6TZnt95yRhwh9inkkSZFFxQpep/RCRKUeyuZWAl31W15dLWuBa98791aq/XXm1R7k4nZ4PhmMhq53NxpfOWNvfDucDK6dmtOYfoItpMK6ewI7f02coYtD75sz1r912P9+2hO3d3MzQLT+5cT1cDAZ3147w0lPq7py/q6BxqsFCLysQrNazNvrN31dSvrSUlRNK/NyUVktXXp4A1+iXJp4GJD9DL9zvriDiaMd7F2MR9feTe/8qtev9XTR8bILSPqG8CgX8mlx8snObp7tNReLGXaZtkiYLSACvJVLfLVYs4iTwNb3XqnsmPgLzE3ricZ7+rck81tBa0wrOdnalpRV8O0HNaDrajSUWb5VVSZ72UrSZD/aO2O8wdcaTekrz97BuxyNrjzXOR/vXhxyrN1Xmf1A3wR7A+S+0jowl/HqBkh4mIyQket/d6ppBQ==
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
var eventgridname = 'eg${prefix}${uniqueString(resourceGroup().id)}'

output appname string = appname
output setup_url string = 'https://${appname}.azurewebsites.net/api/setup_github_app'

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

resource eventgrid 'Microsoft.EventGrid/topics@2020-06-01' = {
    name: eventgridname
    location: location
}

resource egsubscription 'Microsoft.EventGrid/eventSubscriptions@2020-06-01' = {
    name: '${eventgridname}fun'
    scope: eventgrid
    properties: {
      destination: {
        endpointType: 'AzureFunction'
        properties: {
          resourceId: resourceId('Microsoft.Web/sites/functions', apps.name, 'process')
        }
      }
    }
    dependsOn: [
      apps
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
          name: 'EventGridUri'
          value: eventgrid.properties.endpoint
        }
        {
          name: 'EventGridKey'
          value: listKeys(eventgrid.name, '2020-06-01').key1
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
