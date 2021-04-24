// https://aka.ms/bicepdemo#eJyNklFLwzAUhd/zK/IgZAOXrn2SgqAviqAvDhSfJEtvZ9iaxJtkE8f+u8nayLCCQqHJyb0n3z1tUdA3762ri2KpJNgGOsM/q4rvYMmlQeA7pRuzc1yDLwixAkVHN0YKr4ymzqPSK3pJEZwJKOEWTbCTKc8VhGwFUie06CCWMSfO9kGr9wCLY+vkZ6Nqpgd2bGoFdrmtVdj1qrA2i3HpGCHZId5C2YOSaJxpPV94g2IFhevf11KaoL27quZVOZunh0WTPaE0+dUDY9xm9Pp7FcV1TKGmbDB9qljU3DrUR4NsEY+FbgQ2r/ePi1RxIIcTvjTQKeEzLAsHuAVMJ4msvJjNqxFZDuIPtjZomZQYy+90L+WYKWU4YlIeBppynNPwAf4Pc75ROnykqy0aC+gVuEzWj38TB7xr+kHjD9BDfgEuu9iF
// https://aka.ms/bicepdemo#eJyVUm1r2zAQ/u5foQ8FJZDYeYGxeQRWOm90XdMRp+2glKDI50zEkTy9JF2D99t3Suw0rGWjYNDp7tFzzz2+KCI/rC1NHEVzwaHMYKXCx8Eg3MA85EpDuBEyUxsTSrBREJRMsxUpFGdWKEmM1UIuyIhoMMppDp+1cmWrHTaIIFgzTUoNuXhAGB3mw36esz7d5Q2TbAU+b9jJdo+qTrZOip8O0h13629mkbWr/WtWls1zDM3rCIImjSIIvRRcK6NyG6ZWabaAyOzPU86Vk9Z8GPQG/W7PfxQbbgNCfO+4HgGvzcTxIcLkEs2LCa1JbwYUc2bp4h1BQ4FlJjOms9nXSeoRVVAd6fOzHSu8hXlkhAWvqf+223+uqTbmP6JyJ7nPILpTCOkefOtSqxK0FWAajb7VmZK5WDQZ4hukYC26i7C7OkkO5aPRTh+dBlT8Rc1N7QI9Qq1Z4RDGlUR1LfoRcuYKm8isVAJd/6aVVVwVo92Ovq9/xhipR7SD1oe+S4fQpnIBv3yhEMaHpvWEQKvedXtvvFXtcIm1u959uOvePsip/jXIp+vx2fT8apzObq8mF8lkNrkeT88vkxemodzh9qzoa4mT79NknGI4u0km/nyJ+/fwOe998HTDtQG9BvzBVjvYL9MfqJwvMQ==
// https://bicepdemo.z22.web.core.windows.net/

param location string = resourceGroup().location

var prefix = '3f31ffa1'
var saname = 'sa${prefix}${uniqueString(resourceGroup().id)}'
var appname = 'apps${prefix}${uniqueString(resourceGroup().id)}'

resource sa 'Microsoft.Storage/storageAccounts@2021-01-01' = {
  name: saname
  location: location
  kind: 'StorageV2'
  sku: {
    name: 'Standard_LRS'
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
      ]
    }
    reserved: true
  }
}
