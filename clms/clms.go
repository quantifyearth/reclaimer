package clms

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/quantifyearth/reclaimer/internal/utils"
)

type CLMSSearchBatch struct {
	ID    string `json:"@id"`
	First string `json:"first"`
	Last  string `json:"last"`
	Next  string `json:"next"`
}

type CLMSDownloadInfo struct {
	ID         string `json:"@id"`
	Name       string `json:"name"`
	Collection string `json:"collection"`
	// This one is multiformat - can be a string or a struct
	// FullFormat string `json:"full_format"`
	FullPath   string   `json:"full_path"`
	FullSource string   `json:"full_source"`
	Layers     []string `json:"layers"`
}

type CLMSDataset struct {
	ID          string                        `json:"@id"`
	Type        string                        `json:"@type"`
	UID         string                        `json:"UID"`
	Title       string                        `json:"title"`
	Description string                        `json:"description"`
	Downloads   map[string][]CLMSDownloadInfo `json:"dataset_download_information"`
	ReviewState string                        `json:"review_state"`
}

type CLMSSearch struct {
	ID    string          `json:"@id"`
	Batch CLMSSearchBatch `json:"batching"`
	Items []CLMSDataset   `json:"items"`
	Total int             `json:"items_total"`
}

type CLMSFileInfo struct {
	ID         string `json:"@id"`
	Area       string `json:"area"`
	File       string `json:"file"`
	Format     string `json:"format"`
	Path       string `json:"path"`
	Resolution string `json:"resolution"`
	Size       string `json:"size"`
	Source     string `json:"source"`
	Title      string `json:"title"`
	Type       string `json:"type"`
	Version    string `json:"version"`
	Year       string `json:"year"`
}

type CLMSFileList struct {
	Items []CLMSFileInfo `json:"items"`
	// Currently ignoring the schema
}

type CLMSPrepackagedDataset struct {
	ID          string       `json:"@id"`
	Type        string       `json:"@type"`
	UID         string       `json:"UID"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Files       CLMSFileList `json:"downloadable_files"`
	ReviewState string       `json:"review_state"`
}

type CLMSSearchPrepared struct {
	ID    string                   `json:"@id"`
	Batch CLMSSearchBatch          `json:"batching"`
	Items []CLMSPrepackagedDataset `json:"items"`
	Total int                      `json:"items_total"`
}

type CLMSDatumRequest struct {
	DatasetID                    string `json:"DatasetID"`
	DatasetDownloadInformationID string `json:"DatasetDownloadInformationID"`
	OutputFormat                 string `json:"OutputFormat"`
	OutputGCS                    string `json:"OutputGCS"`
}

type CLMSDataRequest struct {
	Datasets []CLMSDatumRequest `json:"Datasets"`
}

type CLMSPreparedDatumRequest struct {
	DatasetID string `json:"DatasetID"`
	FileID    string `json:"FileID"`
}

type CLMSPrepackagedDataRequest struct {
	Datasets []CLMSPreparedDatumRequest `json:"Datasets"`
}

type CLMSTask struct {
	ID string `json:"TaskID"`
}

type CLMSTaskResponse struct {
	TaskIDs      []CLMSTask `json:"TaskIds"`
	ErrorTaskIDs []CLMSTask `json:"ErrorTaskIds"`
}

type CLMSTaskDataset struct {
	DatasetFormat string   `json:"DatasetFormat"`
	DatasetID     string   `json:"DatasetID"`
	DatasetPath   string   `json:"DatasetPath"`
	DatasetSource string   `json:"DatasetSource"`
	DatasetTitle  string   `json:"DatasetTitle"`
	Metadata      []string `json:"Metadata"`
	NUTSID        string   `json:"NUTSID"`
	NUTSName      string   `json:"NUTSName"`
	OutputFormat  string   `json:"OutputFormat"`
	OutputGCS     string   `json:"OutputGCS"`
	WekeoChoices  string   `json:"WekeoChoices"`
}

type CLMSTaskStatus struct {
	DownloadURL string            `json:"DownloadURL"`
	FileSize    int64             `json:"FileSize"`
	UserID      string            `json:"UserID"`
	Status      string            `json:"Status"`
	Message     string            `json:"Message"`
	Datasets    []CLMSTaskDataset `json:"Datasets"`
}

const TaskInProgress = "In_progress"
const TaskFinished = "Finished_ok"

const baseURL = "https://land.copernicus.eu/api/"
const searchPathTemplate = "%s@search?b_start=%d&portal_type=DataSet&metadata_fields=UID&metadata_fields=dataset_full_format&&metadata_fields=dataset_download_information"
const preparedSearchPathTemplate = "%s@search?b_start=%d&portal_type=DataSet&metadata_fields=UID&metadata_fields=downloadable_files"

func fetchIndexBatch(url string, batch interface{}) error {

	headers := map[string]string{
		"Accept": "application/json",
	}
	resp, err := utils.HTTPGet(url, headers)
	if nil != err {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		r, err := io.ReadAll(resp.Body)
		body := resp.Status
		if nil == err {
			body = string(r)
		}
		return fmt.Errorf("unexpected HTTP status %d: %s", resp.StatusCode, body)
	}

	err = json.NewDecoder(resp.Body).Decode(&batch)
	if nil != err {
		return fmt.Errorf("failed to decode response for %s: %w", url, err)
	}
	return nil
}

func FetchIndexGeneratedData() ([]CLMSDataset, error) {
	items := make([]CLMSDataset, 0)

	url := fmt.Sprintf(searchPathTemplate, baseURL, 0)
	for {
		var batch CLMSSearch
		err := fetchIndexBatch(url, &batch)
		if nil != err {
			return nil, err
		}
		if len(batch.Items) == 0 {
			break
		}

		items = append(items, batch.Items...)
		if batch.Batch.Last == url {
			break
		}
		if batch.Batch.Next == url {
			break
		}
		url = batch.Batch.Next
		if "" == url {
			return nil, fmt.Errorf("got invald response: no next URL")
		}
	}
	return items, nil
}

func FetchIndexPrepackagedData() ([]CLMSPrepackagedDataset, error) {
	items := make([]CLMSPrepackagedDataset, 0)

	url := fmt.Sprintf(preparedSearchPathTemplate, baseURL, 0)
	for {
		var batch CLMSSearchPrepared
		err := fetchIndexBatch(url, &batch)
		if nil != err {
			return nil, err
		}
		if len(batch.Items) == 0 {
			break
		}

		items = append(items, batch.Items...)
		if batch.Batch.Last == url {
			break
		}
		if batch.Batch.Next == url {
			break
		}
		url = batch.Batch.Next
		if "" == url {
			return nil, fmt.Errorf("got invald response: no next URL")
		}
	}
	return items, nil
}

func requestData(sessionToken string, payload interface{}) (CLMSTaskResponse, error) {

	jsonStrBytes, err := json.Marshal(payload)
	if nil != err {
		return CLMSTaskResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}
	fmt.Printf("request bytes: %s\n", string(jsonStrBytes))

	url := fmt.Sprintf("%s@datarequest_post", baseURL)

	auth := fmt.Sprintf("Bearer %s", sessionToken)
	headers := map[string]string{
		"Accept":        "application/json",
		"Content-type":  "application/json",
		"Authorization": auth,
	}
	resp, err := utils.HTTPPost(url, headers, string(jsonStrBytes))
	if nil != err {
		return CLMSTaskResponse{}, fmt.Errorf("failed request: %w", err)
	}
	defer resp.Body.Close()

	if http.StatusCreated != resp.StatusCode {
		r, err := io.ReadAll(resp.Body)
		body := resp.Status
		if nil == err {
			body = string(r)
		}
		return CLMSTaskResponse{}, fmt.Errorf("unexpected request response HTTP status %d: %s", resp.StatusCode, body)
	}

	// hopefully we have a task ID now
	var taskResp CLMSTaskResponse
	err = json.NewDecoder(resp.Body).Decode(&taskResp)
	if nil != err {
		return CLMSTaskResponse{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return taskResp, nil
}

func RequestGeneratedData(
	uid string,
	downloadID string,
	outputFormat string,
	coordinateSystem string,
	sessionToken string,
	outputPath string,
) (CLMSTaskResponse, error) {
	// Prep the request
	request := CLMSDatumRequest{
		DatasetID:                    uid,
		DatasetDownloadInformationID: downloadID,
		OutputFormat:                 outputFormat,
		OutputGCS:                    coordinateSystem,
	}
	outerRequest := CLMSDataRequest{
		Datasets: []CLMSDatumRequest{request},
	}

	return requestData(sessionToken, outerRequest)
}

func RequestPrepackagedData(
	uid string,
	downloadID string,
	sessionToken string,
	outputPath string,
) (CLMSTaskResponse, error) {
	// Prep the request
	request := CLMSPreparedDatumRequest{
		DatasetID: uid,
		FileID:    downloadID,
	}
	outerRequest := CLMSPrepackagedDataRequest{
		Datasets: []CLMSPreparedDatumRequest{request},
	}

	return requestData(sessionToken, outerRequest)
}

func RequestDirectData(
	uid string,
	downloadID string,
	sessionToken string,
	outputPath string,
) ([]string, error) {

	url := fmt.Sprintf("%s@get-download-file-urls?dataset_uid=%s&download_information_id=%s", baseURL, uid, downloadID)

	auth := fmt.Sprintf("Bearer %s", sessionToken)
	headers := map[string]string{
		"Accept":        "application/json",
		"Authorization": auth,
	}
	resp, err := utils.HTTPGet(url, headers)
	if nil != err {
		return nil, fmt.Errorf("failed request: %w", err)
	}
	defer resp.Body.Close()

	if http.StatusOK != resp.StatusCode {
		r, err := io.ReadAll(resp.Body)
		body := resp.Status
		if nil == err {
			body = string(r)
		}
		return nil, fmt.Errorf("unexpected request response HTTP status %d: %s", resp.StatusCode, body)
	}

	var downloadURLs []string
	err = json.NewDecoder(resp.Body).Decode(&downloadURLs)
	if nil != err {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return downloadURLs, nil
}

func GetTaskStatus(taskID string, sessionToken string) (CLMSTaskStatus, error) {
	url := fmt.Sprintf("%s@datarequest_status_get?TaskID=%s", baseURL, taskID)
	auth := fmt.Sprintf("Bearer %s", sessionToken)
	headers := map[string]string{
		"Accept":        "application/json",
		"Content-type":  "application/json",
		"Authorization": auth,
	}
	resp, err := utils.HTTPGet(url, headers)
	if nil != err {
		return CLMSTaskStatus{}, fmt.Errorf("failed request: %w", err)
	}
	defer resp.Body.Close()

	if http.StatusOK != resp.StatusCode {
		r, err := io.ReadAll(resp.Body)
		body := resp.Status
		if nil == err {
			body = string(r)
		}
		return CLMSTaskStatus{}, fmt.Errorf("unexpected request HTTP status %d: %s", resp.StatusCode, body)
	}

	// hopefully we have a task ID now
	var taskResp CLMSTaskStatus
	err = json.NewDecoder(resp.Body).Decode(&taskResp)
	if nil != err {
		return CLMSTaskStatus{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return taskResp, nil
}

func GetRequests(
	sessionToken string,
) (map[string]CLMSTaskStatus, error) {
	url := fmt.Sprintf("%s@datarequest_search", baseURL)
	auth := fmt.Sprintf("Bearer %s", sessionToken)
	headers := map[string]string{
		"Accept":        "application/json",
		"Authorization": auth,
	}
	resp, err := utils.HTTPGet(url, headers)
	if nil != err {
		return nil, fmt.Errorf("failed request: %w", err)
	}
	defer resp.Body.Close()

	if http.StatusOK != resp.StatusCode {
		r, err := io.ReadAll(resp.Body)
		body := resp.Status
		if nil == err {
			body = string(r)
		}
		return nil, fmt.Errorf("unexpected request HTTP status %d: %s", resp.StatusCode, body)
	}

	var taskResp map[string]CLMSTaskStatus
	err = json.NewDecoder(resp.Body).Decode(&taskResp)
	if nil != err {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return taskResp, nil
}
