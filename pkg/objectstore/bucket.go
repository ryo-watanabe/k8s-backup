package objectstore

import (
	"os"
	"fmt"
	"time"
	"net/http"
	"crypto/tls"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"

	"k8s.io/klog"
)

type Objectstore interface {
	ChkBucket() (bool, error)
	Upload(file *os.File, filename string) error
	Download(file *os.File, filename string) error
	Delete(filename string) error
	GetObjectInfo(filename string) (*ObjectInfo, error)
	ListObjectInfo() ([]ObjectInfo, error)
}

type ObjectInfo struct {
	Name string
	Size int64
	Timestamp time.Time
	BucketConfigName string
}

type Bucket struct {
	Name string
	AccessKey string
	SecretKey string
	Endpoint string
	Region string
	BucketName string
	insecure bool
}

func NewBucket(name, accessKey, secretKey, endpoint, region, bucketName string, insecure bool) *Bucket {
	return &Bucket{
		Name: name,
		AccessKey: accessKey,
		SecretKey: secretKey,
		Endpoint: endpoint,
		Region: region,
		BucketName: bucketName,
		insecure: insecure,
	}
}

func (b *Bucket) setSession() (*session.Session, error) {
	creds := credentials.NewStaticCredentials(b.AccessKey, b.SecretKey, "")
	var client *http.Client
	if b.insecure {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client = &http.Client{Transport: tr}
	} else {
		client = http.DefaultClient
	}
	sess, err := session.NewSession(&aws.Config{
		HTTPClient: client,
		Credentials: creds,
		Region: aws.String(b.Region),
		Endpoint: &b.Endpoint,
	})
	if err != nil {
		return nil, err
	}
	return sess, nil
}

func (b *Bucket) ChkBucket() (bool, error) {
	// set session
	sess, err := b.setSession()
	if err != nil {
		return false, err
	}

	// list buckets
	svc := s3.New(sess)
	result, err := svc.ListBuckets(nil)
	if err != nil {
		return false, err
	}
	klog.Info("Buckets:")
	found := false
	for _, bu := range result.Buckets {
	    klog.Infof("-- %s created on %s\n", aws.StringValue(bu.Name), aws.TimeValue(bu.CreationDate))
	    if aws.StringValue(bu.Name) == b.BucketName {
		    found = true
	    }
	}

	return found, nil
}

func (b *Bucket) Upload(file *os.File, filename string) error {
	// set session
	sess, err := b.setSession()
	if err != nil {
		return err
	}

	uploader := s3manager.NewUploader(sess)
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(b.BucketName),
		Key: aws.String(filename),
		Body: file,
	})
	if err != nil {
		return fmt.Errorf("Error uploading %s to bucket %s : %s", filename, b.BucketName, err.Error())
	}

	return nil
}

func (b *Bucket) Download(file *os.File, filename string) error {
	// set session
	sess, err := b.setSession()
	if err != nil {
		return err
	}

	downloader := s3manager.NewDownloader(sess)
	_, err = downloader.Download(file,
		&s3.GetObjectInput{
			Bucket: aws.String(b.BucketName),
			Key:    aws.String(filename),
		})
	if err != nil {
		return fmt.Errorf("Error downloading %s from bucket %s : %s", filename, b.BucketName, err.Error())
	}

	return nil
}

func (b *Bucket) Delete(filename string) error {
	// set session
	sess, err := b.setSession()
	if err != nil {
		return err
	}

	svc := s3.New(sess)
	_, err = svc.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(b.BucketName),
		Key: aws.String(filename),
	})
	if err != nil {
		return fmt.Errorf("Error deleting %s from bucket %s : %s", filename, b.BucketName, err.Error())
	}

	err = svc.WaitUntilObjectNotExists(&s3.HeadObjectInput{
	    Bucket: aws.String(b.BucketName),
	    Key:    aws.String(filename),
	})
	return err
}

func (b *Bucket) GetObjectInfo(filename string) (*ObjectInfo, error) {
	// set session
	sess, err := b.setSession()
	if err != nil {
		return nil, err
	}

	// list objects
	svc := s3.New(sess)
	result, err := svc.ListObjects(&s3.ListObjectsInput{
		Bucket: aws.String(b.BucketName),
		Prefix: aws.String(filename),
	})
	if err != nil {
		return nil, err
	}

	// find in list
	for _, obj := range result.Contents {
	    if aws.StringValue(obj.Key) == filename {
		    objInfo := ObjectInfo{
			    Name: filename,
			    Size: aws.Int64Value(obj.Size),
			    Timestamp: aws.TimeValue(obj.LastModified),
			    BucketConfigName: b.Name,
		    }
		    return &objInfo, nil
	    }
	}

	return nil, fmt.Errorf("Object %s not found in bucket %s.", filename, b.BucketName)
}

func (b *Bucket) ListObjectInfo() ([]ObjectInfo, error) {
	// set session
	sess, err := b.setSession()
	if err != nil {
		return nil, err
	}

	// list objects
	svc := s3.New(sess)
	result, err := svc.ListObjects(&s3.ListObjectsInput{
		Bucket: aws.String(b.BucketName),
	})
	if err != nil {
		return nil, err
	}

	// make ObjectInfo list
	objInfoList := make([]ObjectInfo, 0)
	for _, obj := range result.Contents {
		objInfo := ObjectInfo{
			Name: aws.StringValue(obj.Key),
			Size: aws.Int64Value(obj.Size),
			Timestamp: aws.TimeValue(obj.LastModified),
			BucketConfigName: b.Name,
		}
		objInfoList = append(objInfoList, objInfo)
	}

	return objInfoList, nil
}
