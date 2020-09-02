package main

import (
	//"bufio"
	"encoding/json"
	"net/http"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/Azure/azure-storage-blob-go/azblob"
)

// Azure Storage Quickstart Sample - Demonstrate how to upload, list, download, and delete blobs.
//
// Documentation References:
// - What is a Storage Account - https://docs.microsoft.com/azure/storage/common/storage-create-storage-account
// - Blob Service Concepts - https://docs.microsoft.com/rest/api/storageservices/Blob-Service-Concepts
// - Blob Service Go SDK API - https://godoc.org/github.com/Azure/azure-storage-blob-go
// - Blob Service REST API - https://docs.microsoft.com/rest/api/storageservices/Blob-Service-REST-API
// - Scalability and performance targets - https://docs.microsoft.com/azure/storage/common/storage-scalability-targets
// - Azure Storage Performance and Scalability checklist https://docs.microsoft.com/azure/storage/common/storage-performance-checklist
// - Storage Emulator - https://docs.microsoft.com/azure/storage/common/storage-use-emulator

type Blobs struct {
	Container   string
	Blobs []string
}

func randomString() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return strconv.Itoa(r.Int())
}

func handleErrors(err error) {
	if err != nil {
		if serr, ok := err.(azblob.StorageError); ok { // This error is a Service-specific
			switch serr.ServiceCode() { // Compare serviceCode to ServiceCodeXxx constants
			case azblob.ServiceCodeContainerAlreadyExists:
				fmt.Println("Received 409. Container already exists")
				return
			}
		}
		log.Fatal(err)
	}
}

func azureBlobOperation(w http.ResponseWriter, r *http.Request) {

	// From the Azure portal, get your storage account name and key and set environment variables.
	accountName, accountKey, containerName := os.Getenv("storageAccountName"), os.Getenv("accessKey"), os.Getenv("containerName")
	if len(accountName) == 0 || len(accountKey) == 0 || len(containerName) == 0 {
		log.Fatal("storageAccountName or accessKey or containerName environment variable is not set")
	}

	// Create a default request pipeline using your storage account name and account key.
	credential, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		log.Fatal("Invalid credentials with error: " + err.Error())
	}
	p := azblob.NewPipeline(credential, azblob.PipelineOptions{})

	// From the Azure portal, get your storage account blob service URL endpoint.
	URL, _ := url.Parse(
		fmt.Sprintf("https://%s.blob.core.windows.net/%s", accountName, containerName))

	// Create a ContainerURL object that wraps the container URL and a request
	// pipeline to make requests.
	containerURL := azblob.NewContainerURL(*URL, p)
	ctx := context.Background()

	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case "GET":
		fmt.Println("Listing the blobs in the container:")
		var blobItems []string
		for marker := (azblob.Marker{}); marker.NotDone(); {
			// Get a result segment starting with the blob indicated by the current Marker.
			listBlob, err := containerURL.ListBlobsFlatSegment(ctx, marker, azblob.ListBlobsSegmentOptions{})
			handleErrors(err)

			// ListBlobs returns the start of the next segment; you MUST use this to get
			// the next segment (after processing the current result segment).
			marker = listBlob.NextMarker

			// Process the blobs returned in this result segment (if the segment is empty, the loop body won't execute)
			for _, blobInfo := range listBlob.Segment.BlobItems {
				//fmt.Print("	Blob name: " + blobInfo.Name + "\n")
				blobItems = append(blobItems, blobInfo.Name)
			}
		}
		blobs := Blobs{containerName, blobItems}
		encjson, _ := json.Marshal(blobs)
		fmt.Println(string(encjson))
		w.WriteHeader(http.StatusOK)
		w.Write(encjson)
	case "POST":
		// Create a file to test the upload and download.
		fmt.Printf("Creating a dummy file to test the upload and download\n")
		data := []byte("hello world this is a blob\n")
		fileName := randomString()
		err = ioutil.WriteFile(fileName, data, 0700)
		handleErrors(err)

		// Here's how to upload a blob.
		blobURL := containerURL.NewBlockBlobURL(fileName)
		file, err := os.Open(fileName)
		handleErrors(err)

		fmt.Printf("Uploading the file with blob name: %s\n", fileName)
		_, err = azblob.UploadFileToBlockBlob(ctx, file, blobURL, azblob.UploadToBlockBlobOptions{
			BlockSize:   4 * 1024 * 1024,
			Parallelism: 16})
		handleErrors(err)

		w.WriteHeader(http.StatusCreated)
		file.Close()
		w.Write([]byte(`{"message": "file `+ fileName +` is uploaded"}`))
	//case "PUT":
	//	w.WriteHeader(http.StatusAccepted)
	//	w.Write([]byte(`{"message": "put called"}`))
	case "DELETE":
		reqFilename := r.URL.Query().Get("filename")
		fmt.Printf("Deleting the file with blob name: %s\n", reqFilename)
		blobURL := containerURL.NewBlockBlobURL(reqFilename)
		blobURL.Delete(ctx, "include", azblob.BlobAccessConditions{})
	//	fmt.Printf("Cleaning up.\n")
	//	containerURL.Delete(ctx, azblob.ContainerAccessConditions{})
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "file `+ reqFilename +` is deleted"}`))
	default:
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message": "not found"}`))
	}
}

func main() {

	fmt.Printf("Starting server at port 8080\n")
	http.HandleFunc("/azblob", azureBlobOperation)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}

}