package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Torrent struct {
	InfoHash string
	Name     string
	Percent  float32
}

type Torrents map[string]Torrent

var (
	host     = "http://cloud-torrent-server:3000"
	path     = "/nas/downloads/"
	username = "admin"
	password = "password"
)

func main() {

	for {
		torrents, err := GetTorrents()
		if err != nil {
			fmt.Println(err)
		}
		Worker(torrents)
		time.Sleep(10 * time.Second)
		fmt.Println("next loop")
	}
}

func Worker(torrents Torrents) {
	for k, v := range torrents {
		if v.Percent == 100 {
			fmt.Println(v.Name)
			fmt.Println("Start Download: " + v.Name)
			fileUrl := host + "/download/" + v.Name
			torrentUrl := host + "/api/torrent"
			DownloadFile(v.Name, fileUrl)
			fmt.Println("Downloaded: " + fileUrl)
			go Unzip(v.Name)
			fmt.Println("Start Delete Remote File: " + fileUrl)
			err := DeleteFile(fileUrl)
			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Println("Start Delete Torrent: " + k)
			err = DeleteTorrent(torrentUrl, k)
			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Println("Deleted Torrent: " + fileUrl)
		}
	}
}

func GetTorrents() (Torrents, error) {
	var torrents Torrents
	client := &http.Client{}

	req, err := http.NewRequest("GET", host+"/torrent/status", nil)
	if err != nil {
		fmt.Println(err)
	}
	req.SetBasicAuth(username, password)
	resp, err := client.Do(req)
	// resp, err := http.Get(host + "/torrent/status")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(body, &torrents)
	fmt.Println(torrents)
	return torrents, nil
}

func DownloadFile(filename string, url string) error {
	start := time.Now()
	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Println(err)
	}
	req.SetBasicAuth(username, password)
	resp, err := client.Do(req)
	// resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	fmt.Println("create file ", filename)
	out, err := os.Create(path + filename)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	fmt.Printf("%s download took %v\n", filename, time.Since(start))
	return err
}

func DeleteFile(url string) error {
	client := &http.Client{}

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		fmt.Println(err)
		return err
	}
	req.SetBasicAuth(username, password)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer resp.Body.Close()
	return nil
}

func DeleteTorrent(url, InfoHash string) error {
	client := &http.Client{}

	req, err := http.NewRequest("POST", url, strings.NewReader("delete:"+InfoHash))
	if err != nil {
		fmt.Println(err)
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(username, password)
	_, err = client.Do(req)
	// _, err := http.Post(url, "application/x-www-form-urlencoded", strings.NewReader("delete:"+InfoHash))
	if err != nil {
		return err
	}
	return nil
}

func Unzip(filename string) error {
	source := path + filename
	fmt.Println("unzip file:", source)
	destination := source + "_unzip"
	if err := os.Mkdir(destination, 0777); err != nil {
		fmt.Println(err)
		return err
	}
	if err := unzipSource(source, destination); err != nil {
		return err
	}
	return nil
}

func unzipSource(source, destination string) error {
	reader, err := zip.OpenReader(source)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer reader.Close()

	// Iterate over zip files inside the archive and unzip each of them
	for _, f := range reader.File {
		err := unzipFile(f, destination)
		if err != nil {
			return err
		}
	}
	os.Remove(source)
	return nil
}

func unzipFile(f *zip.File, destination string) error {
	// Check if file paths are not vulnerable to Zip Slip
	filePath := filepath.Join(destination, f.Name)
	if !strings.HasPrefix(filePath, filepath.Clean(destination)+string(os.PathSeparator)) {
		return fmt.Errorf("invalid file path: %s", filePath)
	}

	// Create directory tree
	if f.FileInfo().IsDir() {
		if err := os.MkdirAll(filePath, os.ModePerm); err != nil {
			return err
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
		return err
	}

	// Create a destination file for unzipped content
	destinationFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	// Unzip the content of a file and copy it to the destination file
	zippedFile, err := f.Open()
	if err != nil {
		return err
	}
	defer zippedFile.Close()

	if _, err := io.Copy(destinationFile, zippedFile); err != nil {
		return err
	}
	return nil
}
