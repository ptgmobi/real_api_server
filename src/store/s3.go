package store

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

var (
	svc            *s3.S3
	svcOnce        sync.Once
	downloader     *s3manager.Downloader
	downloaderOnce sync.Once

	gDownloadManager *downloadManager = NewDownloadManager()
)

type downloadManager struct {
	sync.RWMutex
	downloadMap map[string]bool
}

func NewDownloadManager() *downloadManager {
	return &downloadManager{
		downloadMap: make(map[string]bool, 32),
	}
}

func (m *downloadManager) setDownload(key string) {
	m.Lock()
	defer m.Unlock()
	m.downloadMap[key] = true
}

func (m *downloadManager) cleanDownload(key string) {
	m.Lock()
	defer m.Unlock()
	m.downloadMap[key] = false
}

func (m *downloadManager) shouldDownload(key string) bool {
	m.RLock()
	defer m.RUnlock()
	return !m.downloadMap[key]
}

// 获取文件夹中的文件目录
func ListFolder(region, path, bucket, prefix string) ([]string, error) {
	svcOnce.Do(func() {
		svc = s3.New(session.New(aws.NewConfig().WithRegion(region)))
	})

	if svc == nil {
		return nil, errors.New("svc is nil")
	}

	input := &s3.ListObjectsInput{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	}

	// 获取文件后，创建临时需要的文件夹
	if err := os.MkdirAll(path+prefix, 0755); err != nil {
		return nil, err
	}

	result, err := svc.ListObjects(input)
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(result.Contents))
	for _, obj := range result.Contents {
		keys = append(keys, *obj.Key)
	}
	return keys, nil
}

// 下载文件
func DownloadFile(region, path, bucket, key string) ([]byte, error) {
	downloaderOnce.Do(func() {
		sess := session.Must(session.NewSession(aws.NewConfig().WithRegion(region)))
		downloader = s3manager.NewDownloader(sess)
	})

	if !gDownloadManager.shouldDownload(key) {
		return nil, errors.New(key + " is downloading now")
	}

	if downloader == nil {
		return nil, errors.New("downloader nil")
	}

	gzipEnabled := false
	if strings.HasSuffix(key, ".gz") {
		gzipEnabled = true
	}

	destTmpFile := path + key + ".tmp"

	f, err := os.Create(destTmpFile)
	if err != nil {
		return nil, err
	}
	defer func() {
		f.Close()
		os.Remove(destTmpFile)
	}()

	gDownloadManager.setDownload(key)
	defer gDownloadManager.cleanDownload(key)

	_, err = downloader.Download(f, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}

	if gzipEnabled {
		f.Seek(0, os.SEEK_SET)
		gr, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer gr.Close()

		var out bytes.Buffer
		if _, err = io.Copy(&out, gr); err != nil {
			return nil, err
		}

		return out.Bytes(), nil
	}

	return ioutil.ReadAll(f)
}
