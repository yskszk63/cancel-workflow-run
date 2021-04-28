package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Azure/azure-storage-blob-go/azblob"
)

func TestParseConnectString(t *testing.T) {
	input := "AccountName=name;AccountKey=cGFzcwo="

	name, key, err := parseConnectionString(input)
	if err != nil {
		t.Fatal(err)
	}
	if *name != "name" || *key != "cGFzcwo=" {
		t.Fatal("name or key not match")
	}
}

func TestParseConnectStringWithNoAccountName(t *testing.T) {
	input := "AccountKey=key"

	_, _, err := parseConnectionString(input)
	if err == nil {
		t.Fail()
	}
}

func TestParseConnectStringWithNoAccountKey(t *testing.T) {
	input := "AccountKey=cGFzcwo="

	_, _, err := parseConnectionString(input)
	if err == nil {
		t.Fail()
	}
}

func TestNewAzblobCredential(t *testing.T) {
	input := "AccountName=name;AccountKey=cGFzcwo="

	c, err := newAzblobCredential(input)
	if err != nil {
		t.Fatal(err)
	}
	if c.AccountName() != "name" {
		t.Fail()
	}
}

func TestNewAzblobCredentialMissingKey(t *testing.T) {
	input := "AccountName=name"

	_, err := newAzblobCredential(input)
	if err == nil {
		t.Fail()
	}
}

func TestNewAzblobCredentialWithIncorrectKey(t *testing.T) {
	input := "AccountName=name;AccountKey=plain"

	_, err := newAzblobCredential(input)
	if err == nil {
		t.Fail()
	}
}

func TestEnsureContainer(t *testing.T) {
	dummy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
	}))
	defer dummy.Close()

	cred, err := newAzblobCredential("AccountName=name;AccountKey=cGFzcwo=")
	if err != nil {
		t.Fatal(err)
	}
	url, err := ensureContainer(context.Background(), cred, "container", dummy.URL+"/%s/%s")
	if err != nil {
		t.Fatal(err)
	}
	if u := url.URL(); u.String() != dummy.URL+"/name/container" {
		t.Fail()
	}
}

func TestEnsureContainerAlreadyExists(t *testing.T) {
	dummy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("x-ms-error-code", "ContainerAlreadyExists")
		w.WriteHeader(409)
	}))
	defer dummy.Close()

	cred, err := newAzblobCredential("AccountName=name;AccountKey=cGFzcwo=")
	if err != nil {
		t.Fatal(err)
	}
	_, err = ensureContainer(context.Background(), cred, "container", dummy.URL+"/%s/%s")
	if err != nil {
		t.Fatal(err)
	}
}

func TestEnsureContainerOtherError(t *testing.T) {
	dummy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("x-ms-error-code", "ContainerDisabled")
		w.WriteHeader(409)
	}))
	defer dummy.Close()

	cred, err := newAzblobCredential("AccountName=name;AccountKey=cGFzcwo=")
	if err != nil {
		t.Fatal(err)
	}
	_, err = ensureContainer(context.Background(), cred, "container", dummy.URL+"/%s/%s")
	if err == nil {
		t.Fail()
	}
}

func TestEnsureContainerInvalidURL(t *testing.T) {
	cred, err := newAzblobCredential("AccountName=name;AccountKey=cGFzcwo=")
	if err != nil {
		t.Fatal(err)
	}
	_, err = ensureContainer(context.Background(), cred, "container", "")
	if err == nil {
		t.Fail()
	}
}

func TestNewBlobUrlWithSas(t *testing.T) {
	u, err := url.Parse("http://localhost/")
	if err != nil {
		t.Fatal(err)
	}
	conurl := azblob.NewContainerURL(*u, azblob.NewPipeline(azblob.NewAnonymousCredential(), azblob.PipelineOptions{}))

	cred, err := newAzblobCredential("AccountName=name;AccountKey=cGFzcwo=")
	if err != nil {
		t.Fatal(err)
	}

	b := conurl.NewBlobURL("test")
	bloburl, err := newBlobUrlWithSas(cred, &b, 10, true, true)
	if err != nil {
		t.Fatal(err)
	}
	rawurl := bloburl.URL()
	query := rawurl.Query()
	if query.Get("sp") != "rw" || query.Get("spr") != "https" {
		t.Fail()
	}
}

func TestNewBlobUrlWithSasNoCredential(t *testing.T) {
	u, err := url.Parse("http://localhost/")
	if err != nil {
		t.Fatal(err)
	}
	conurl := azblob.NewContainerURL(*u, azblob.NewPipeline(azblob.NewAnonymousCredential(), azblob.PipelineOptions{}))

	bloburl := conurl.NewBlobURL("test")
	_, err = newBlobUrlWithSas(nil, &bloburl, 10, true, true)
	if err == nil {
		t.Fail()
	}
}

func TestExistsBlob(t *testing.T) {
	dummy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("x-ms-error-code", "BlobNotFound")
		w.WriteHeader(404)
	}))
	defer dummy.Close()

	url, err := url.Parse(dummy.URL)
	if err != nil {
		t.Fatal(err)
	}
	blob := azblob.NewBlobURL(*url, azblob.NewPipeline(azblob.NewAnonymousCredential(), azblob.PipelineOptions{}))
	exists, err := existsBlob(context.Background(), &blob)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fail()
	}
}

func TestExistsBlobExists(t *testing.T) {
	dummy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer dummy.Close()

	url, err := url.Parse(dummy.URL)
	if err != nil {
		t.Fatal(err)
	}
	blob := azblob.NewBlobURL(*url, azblob.NewPipeline(azblob.NewAnonymousCredential(), azblob.PipelineOptions{}))
	exists, err := existsBlob(context.Background(), &blob)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fail()
	}
}

func TestExistsBlobErr(t *testing.T) {
	dummy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("x-ms-error-code", "ContainerDisabled")
		w.WriteHeader(409)
	}))
	defer dummy.Close()

	url, err := url.Parse(dummy.URL)
	if err != nil {
		t.Fatal(err)
	}
	blob := azblob.NewBlobURL(*url, azblob.NewPipeline(azblob.NewAnonymousCredential(), azblob.PipelineOptions{}))
	_, err = existsBlob(context.Background(), &blob)
	if err == nil {
		t.Fail()
	}
}

func TestNewBlobUrlFromSas(t *testing.T) {
	u := "http://example.org/"
	_, err := newBlobUrlFromSas(u)
	if err != nil {
		t.Fatal(err)
	}
}

func TestNewBlobUrlFromSasErr(t *testing.T) {
	u := ":"
	_, err := newBlobUrlFromSas(u)
	if err == nil {
		t.Fail()
	}
}

func TestTouchIfAbsentAndUpdate(t *testing.T) {
	dummy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer dummy.Close()

	blob, err := newBlobUrlFromSas(dummy.URL)
	previous, err := touchIfAbsent(context.Background(), blob)
	if err != nil {
		t.Fatal(err)
	}

	putIfUnmodified(context.Background(), blob, "content", previous)
}
