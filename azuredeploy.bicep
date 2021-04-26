// https://aka.ms/bicepdemo#eJydV21v4jgQ/s6vyIdKAakktHtb3XGqVLabUq5XqAjd3mlVRSYZgpdg52wH+iLut984b4Q27dKLEBh7/Mx4Xp5xbNuYKxXLrm1PqQ9xAEtuPR0fW2uYWj4XYK0pC/haWgyU3bC34gH3pbWkvuCSzxTKLu0fpP0jtslTIiD7bs8S5ivKmbTLUZuymSBSicRXWoTIts8DaDTOApC+oLGWapp9qi6TqdGLY6m/DBpYxihdI5FlTBNlDAECaUREgTBbjZgIsjRIHNPAQHDKQuPUMM3GmQQf1TRbL/C/EAknvxjAtPLAqKq7EXSFqMYVPO6jEz0153zhoSIBai/lVW132XbDTbfvo7BGUWNFhBFxn+i9OCVA8kT40Bc8iZstq1hK5WIBM/qgN/qE+RCtZ8JMFyRhZAl6QZKD50xsc/CcMPpPAm6qrvkSmQatTbYbfV9sx6H8OABlUhUIevz/EBQJI8oWCOJzhqdumnMaBMDaerZrHr7yDQ0ODdOOBV/RAIS0r8ucxtDYkiqQNm7Lj9dKFcEKmAoFDQp7IfyYtQ2eqBjjWzitDGY+UaxLUEnsJSKqhLuowIPnXHhjpdWGiZham5Yqiamt3UGiyAupmidTD6VRcWEPRtswt2d1FRckBFtmvz3f5wlT8uy4c3zU7uiPibqfG4ahNXbzXMG/RWp1yxFOLpA0uoaZg347NnFOLpJuClBA4DJhARGB9+fY1RKbxqZi3zTi0z0stLWcC2KF9FVnbqnt4DmzeWMHMCNJpMx0FQMfg1AUZGGdgZkjKv+y/+Mk0iLfy0mjIqAf9DRfQzASNETH70rqp4xbzAWGJYuZ5k1zR/C+DvQa1JwHdaB9Z/Le/iV56IUwYMgunGmAo51leIi5hOASiE59hK/VXr+8KcfFbDaTfQcQAyocsa3NkjQy6WqUy0qqhtrRk32ctBWPqZ+GtdPunNSEdacS0/mahNxRGMpkWnJxvdYU1K2IvWcBJtaOERtsdVlIpI+pVTHxzXzD5qAoy63eZhU6MOaUqcmjhjF7OmEu8j66DXodoH6KEw+wErfjplnHb2V3zogOKQQPgrSI2FhVElvPy6j/JNYapCbampGqLh8wScM5FjGWQcwZZIxz9Lnd+fyKcYr28BPOQRbUvsE2UPoDI1TpDBsTpca5RRnt1DkRm3NEM3AvD0AOvUtT+qRGnVP1QX5tH72mzoLi3z9HERKUPkSzkwez1kyt6pyzGQ230cctLijMqHCHMarJkadumlJo8R98KnN+rbLJikQJihWN9GvGm06elvJGcMV9Hp2mzPZ7zshDhD7FPJKkyKJiBS9UeiGiUg9lcyuBrvotr66WtcC17517K9X+OvNqD3JxOzyfDEZD17sbja+csTe+HU4G107NaUw/wRZSYd09gZ2/Js7QxaH3zRnr3zrsfz/tidu7uRkgWv9y4no4mIxvr53hpKdVXTl/10Dj5QIEXlehWS3m7QWcvi4lfW0pqqaVebmorJYuPbyDL1EuTTwMyH6G3zlf3MHE0Q72Lsaja++md37V69d6uuh42QUkfUd4lAv5tDj5ZGd3z/aai8UMu0xbJMwWEAHeyyW+XKxZxElg65uvVHZM/AXmpvVE4z39W5L5raA1ppWcbG1Lyir49oMa0HU1Gsos36oqk71sJWmyH+2dMd7ga42m9KVn7+BdjkZXnuucj3cvDjnW7svMfqBvgr0Bcl9pHZjLeHUDJDxMRsjI9T+AE2nL
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
