package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/Azure/azure-storage-blob-go/azblob"
)

func parseConnectionString(s string) (*string, *string, error) {
	var account *string
	var key *string
	for _, kv := range strings.Split(s, ";") {
		kv := strings.SplitN(kv, "=", 2)
		k, v := kv[0], kv[1]
		switch k {
		case "AccountName":
			account = &v
		case "AccountKey":
			key = &v
		}
	}
	if account == nil || key == nil {
		return nil, nil, fmt.Errorf("no AccountName or AccountKey")
	}
	return account, key, nil
}

func newAzblobCredential(s string) (*azblob.SharedKeyCredential, error) {
	account, key, err := parseConnectionString(s)
	if err != nil {
		return nil, err
	}

	cred, err := azblob.NewSharedKeyCredential(*account, *key)
	if err != nil {
		return nil, err
	}

	return cred, nil
}

const defaultContainerTemplate = "https://%s.blob.core.windows.net/%s"

func ensureContainer(context context.Context, cred *azblob.SharedKeyCredential, container string, urlTemplate string) (*azblob.ContainerURL, error) {
	rawurl, err := url.Parse(fmt.Sprintf(urlTemplate, cred.AccountName(), container))
	if err != nil {
		return nil, err
	}

	conurl := azblob.NewContainerURL(*rawurl, azblob.NewPipeline(cred, azblob.PipelineOptions{}))

	_, err = conurl.Create(context, azblob.Metadata{}, azblob.PublicAccessNone)
	if err != nil {
		if err, ok := err.(azblob.StorageError); ok && err.ServiceCode() == azblob.ServiceCodeContainerAlreadyExists {
			// pass
		} else {
			return nil, err
		}
	}
	return &conurl, nil
}

func newBlobUrlWithSas(cred azblob.StorageAccountCredential, blob *azblob.BlobURL, expireInMinutes int, read bool, write bool) (*azblob.BlobURL, error) {
	bloburl := blob.URL()
	parts := azblob.NewBlobURLParts(bloburl)

	sas, err := azblob.BlobSASSignatureValues{
		Protocol:      azblob.SASProtocolHTTPS,
		ExpiryTime:    time.Now().UTC().Add(15 * time.Minute),
		ContainerName: parts.ContainerName,
		BlobName:      parts.BlobName,
		Permissions:   azblob.BlobSASPermissions{Read: read, Write: write}.String(),
	}.NewSASQueryParameters(cred)
	if err != nil {
		return nil, err
	}

	bloburl.RawQuery = sas.Encode()

	u := azblob.NewBlobURL(bloburl, azblob.NewPipeline(azblob.NewAnonymousCredential(), azblob.PipelineOptions{}))
	return &u, nil
}

func existsBlob(context context.Context, blob *azblob.BlobURL) (bool, error) {
	_, err := blob.GetProperties(context, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
	if err != nil {
		if err, ok := err.(azblob.StorageError); ok && err.ServiceCode() == azblob.ServiceCodeBlobNotFound {
			return false, nil
		} else {
			return false, err
		}
	}
	return true, nil
}

func newBlobUrlFromSas(urlwithsas string) (*azblob.BlobURL, error) {
	rawurl, err := url.Parse(urlwithsas)
	if err != nil {
		return nil, err
	}

	result := azblob.NewBlobURL(*rawurl, azblob.NewPipeline(azblob.NewAnonymousCredential(), azblob.PipelineOptions{}))
	return &result, nil
}

func touchIfAbsent(context context.Context, blob *azblob.BlobURL) (*azblob.CommonResponse, error) {
	created, err := azblob.UploadBufferToBlockBlob(context, []byte{}, blob.ToBlockBlobURL(), azblob.UploadToBlockBlobOptions{
		AccessConditions: azblob.BlobAccessConditions{
			ModifiedAccessConditions: azblob.ModifiedAccessConditions{
				// fail if exists
				IfNoneMatch: azblob.ETagAny,
			},
		},
	})
	return &created, err
}

func putIfUnmodified(context context.Context, blob *azblob.BlobURL, content string, previous *azblob.CommonResponse) error {
	_, err := azblob.UploadBufferToBlockBlob(context, []byte(content), blob.ToBlockBlobURL(), azblob.UploadToBlockBlobOptions{
		AccessConditions: azblob.BlobAccessConditions{
			ModifiedAccessConditions: azblob.ModifiedAccessConditions{
				// fail if modified after created
				IfMatch: (*previous).ETag(),
			},
		},
	})
	return err
}
