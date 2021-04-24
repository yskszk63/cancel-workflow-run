// https://aka.ms/bicepdemo#eJzVVl9v2kgQf/en2IdIC1JsE3qNWk6RQlM3oVyAw6S5U1VZiz3AFrPr211Dk4j77DdrY6ANvdJ7OwmZ9c7Mb/7P2PfJzJhMt3x/zGPIElhI77HZ9FYw9mKpwFtxkciV9gQY3/F37ImMtbfgsZJaTgzyLvzPzP2c+ewxV1A+3UkuYsOl0P725HIxUUwblcfGsjDtxjIBx7lMQMeKZ5arRq+5ucnHpJ1l2j4ITzzSL2gs9cg4N6QHkGiSMgOK1p2MKbYgLMt4QhCciym5IJQ6lxpiVFOrf4P/hmk4/4WAsMoTsq9uoPgSUUkXHo7RiZGaSTmPUJECc5TyfW33pTgJC/FjFB5Q5CyZIqmMmZXFKwVa5iqGayXzrFb3KlLBlymY8C9WMGYihnQ1UbQgaCbYAixBs5Onkm198pQL/lcOYaGu9i0yT+rrUhpjX4njUf88ABfaVAj2/N8QDJumXMwRJJYCva7RGU8SEK69bdHTZ7HhySmhfqbkkiegtH+7rWlMja+5Ae2j2Ma9uuNUABgvQnfcoZGKTcHX5X87jmUujL5sNppnbsP+KBr15BBicVqbaONrlZzW9oSXc2y7FqEb0A9Nind6nrcKgAoCyUwkTCXRb8PQcqyd9Z59Nhz7FnaE5tOZ0T42ayYFlNadvXQbL59ZVyXjB/Zh8Vu9GHRdmUZPnvbysKbINdxYVJpIsAJlBspw2AphK6S8BI9GDxnsoL92yVbWvkvbFFlHXrlnz8O8ydsP/KjGE3Kfotn5F3rQTKvqSooJn1Y3xCoIwRisTGT7uLkkW/Jettp2JqLF7+VYbxJL97iWLM2RrSrbtzBheWoCkWSSY6oGShoZy/SimMC/buqrh9AXWJ6aeVYLlnJFwfFlCSnX9qhrOw4M1Wu3cW5DVffmSPvY+OQV2utbc9b/5si7u97VqNPvhdF9f9gNhtHwrjfq3AYHvKFxjg2xoD8LHPwxCnohHqMPwdD+H8L++8WRuO3BoINo1zejMMLDaHh3G/RGbauqG/x5ABoHDyhcDrCdNp2kRnfrjj9vJTskqq6pl1GuOqtuWw833gL5isLDhBxn+H3wJuyMAhvg6N2wfxsN2lfd9vXBSFebecrNLB8XG/lBz/Xj/PyFX056dyXVfJLKlaty4StIAbegxlW+EqlkiW/3jDZ+xuI51qb3yLMj4/t7DnY8FyWN/SGgaKZyXv8/SxwrJuq8PWB78YlxdPJu+v1uFAZXw2B0AOvrT4fjQL8L9h2QT87uDWsZ1BJw4GExQjlc/wGulRSK
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
