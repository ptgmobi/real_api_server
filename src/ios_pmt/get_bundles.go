package ios_pmt

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

var gBundleMap *map[string][]string
var gBundleLock sync.RWMutex
var gMd5Str string

func getBundlesWitdhFile(f string) (*map[string][]string, error) {
	body, err := ioutil.ReadFile(f)
	if err != nil {
		return nil, fmt.Errorf("[getBundlesWithFile] read file err: %v", err)
	}
	m := make(map[string][]string, 200)
	err = json.Unmarshal(body, &m)
	if err != nil {
		return nil, fmt.Errorf("[getBundlesWitdhFile] decode json err: %v", err)
	}
	return &m, nil
}

func getFileMd5(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("[getFileMd5] open file err: %v, filePath: %s", err, filePath)
	}
	defer f.Close()

	md5h := md5.New()
	io.Copy(md5h, f)
	md5Byte := md5h.Sum([]byte(""))
	return string(md5Byte), nil
}

func downloadBundleWithS3(filePath string) error {
	bucket := s3Bucket
	key := s3Key

	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("[downloadBundleWithS3] create file err: %v, filePath: %s", err, filePath)
	}
	defer f.Close()

	sess := session.Must(session.NewSession(aws.NewConfig().WithRegion(awsRegion)))

	downloader := s3manager.NewDownloader(sess)

	_, err = downloader.Download(f, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return fmt.Errorf("[downloadBundleWithS3] download file err: %v, filePath: %s", err, filePath)
	}
	return nil
}

func InitBundle(filePath string) error {
	err := downloadBundleWithS3(filePath)
	if err != nil {
		return err
	}

	gBundleMap, err = getBundlesWitdhFile(filePath)
	if err != nil {
		return err
	}

	gMd5Str, err = getFileMd5(filePath)
	if err != nil {
		return err
	}
	return nil
}

func updateBundleWithLock(filePath string) {
	shortRest := time.Duration(time.Minute * 10)
	defaultTime := time.Hour
	sleepTime := defaultTime

	for {
		time.Sleep(sleepTime)
		if err := downloadBundleWithS3(filePath); err != nil {
			fmt.Println("[updateBundleWithLock] download err: ", err)
			sleepTime = shortRest
			continue
		}

		newMd5, err := getFileMd5(filePath)
		if err != nil {
			fmt.Println("[updateBundleWithLock] get md5 err: ", err)
			sleepTime = shortRest
			continue
		}
		if newMd5 == gMd5Str {
			fmt.Println("[updateBundleWithLock] file have same content")
			continue
		}
		newBundleMap, err := getBundlesWitdhFile(filePath)
		if err != nil {
			fmt.Println("[updateBundleWithLock] get new bundle err: ", err)
			sleepTime = shortRest
			continue
		}

		gBundleLock.Lock()
		gBundleMap = newBundleMap
		gMd5Str = newMd5
		gBundleLock.Unlock()

		sleepTime = defaultTime

		fmt.Println("[updateBundleWithLock] update success, date: ", time.Now())
	}
}

func GetBundlesWithCountry(country string) []string {
	gBundleLock.RLock()
	defer gBundleLock.RUnlock()

	var countries []string
	if gBundleMap == nil {
		fmt.Println("[GetBundlesWithCountry] gBundleMap is nil")
		return countries
	}
	countries = (*gBundleMap)[country]
	return countries
}
