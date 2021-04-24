//https://aka.ms/bicepdemo#eJzVVm1v2kgQ/u5fsR8iGaTYJvQatZwihaZuQrkAh0lzp6qyFnuALWbXty/QJOJ++83aGGhDe/S+nYTMemfmmfcZBwGZaZ2rVhCMWQJ5CgvhPzab/grGfiIk+CvGU7FSPgcdOMGOPRWJ8hcskUKJiUbeRfCZep/zgD4aCeXTmxieaCa4CrYnj/GJpEpLk2jLQpWXiBQc5zIFlUiWW66ae830jRmTdp4r+yAs9Um/oNHMJ2OjSQ8gVSSjGqRbd3Iq6YLQPGcpQXDGp+SCuK5zqSBBNbX6N/hvqILzXwhwqzwl++oGki0RlXTh4RidGKmZEPMYFUnQRynf13ZfipOoED9G4QFFzpJKkomEWlm8kqCEkQlcS2HyWt2vSAVfLmHCvljBhPIEstVEugVBUU4XYAmKnjyVbOuTJ8PZXwaiQl3tW2SW1telNMa+Esej+nkAxpWuEOz5vyFoOs0YnyNIIjh6XXNnLE2Be/a25Z4+iw1LT4kb5FIsWQpSBbfbmsbUBIppUAGKbdyrO44wOsfUVP5u87C5cJxKAwaUuDu4SAtJpxCo8r+dJMJwrS6bjeaZ17A/F1GeHEIsTGuTDnytstfanvByjn3ZIu4G9EPTxTs1N60CoIJAMuUplWn82zCyHGtnvWefjde+hR2u2HSmVYDdnAsOpXVnL73Gy2fWVdn6F/uwO6xezIqqTHNPnvYStXaRa7ixqDSRYImKHKRmsBXCXslYCR6PHnLYQX/tki29fZe2ObSOvPLOnoe5StuP/ajmF3Kfotnmi3vQTKvqSvAJm1Y3xCqIQGssEmT7uLkkW/Jettp2aKLF78VYbRLr7nEtaWaQrarrtzChJtMhT3PBMFUDKbRIRHZRjOhfN/XVQ+gLrF9FfasFa72i4HyzhIwpe1S1HQeG6rXXOLehqvtzpH1sfPIL7fWtOesfOfLurnc16vR7UXzfH3bDYTy86406t+EBb9zEYEMs3J8FDv8Yhb0Ij/GHcGj/D2H//eJI3PZg0EG065tRFONhNLy7DXujtlXVDf88AI2TCSRuD9iOo05ac3f7kD1vJTtFqq6pl1GuOqtuWw9X4gL5isLDhBxn+H34JuqMQhvg+N2wfxsP2lfd9vXBSFere8r0zIyLlf2g5upxfv4iKFeBtxJyPsnEypOGBxIywDWpcNeveCZoGthFpHSQ02SOtek/svzI+P5uwM7voqSxPzgUzVQO9P9niWPFxJ23B2wvvkGOTt5Nv9+No/BqGI4OYH39bXEc6HfBvgPyydm9YS2DXAIOPCxGKIfrP3O/IGc= 
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
