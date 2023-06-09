package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "rest-api-go/docs"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudfront"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

var uploader *s3manager.Uploader

// @title           Rest API Go
// @version         1.0
// @description     This is a sample server for a rest api go includes s3 and cloudfront endpoints.

// @contact.name   Atakan Demircioğlu
// @contact.url    https://twitter.com/atakde

// @host      localhost:8080
// @BasePath  /

func main() {
	LoadEnv()

	r := gin.Default()

	r.GET("/fetch-from-cloud-front", cloudFrontFetchEndpoint)
	r.POST("/upload-image", uploadEndpoint)
	r.PUT("/update-image", updateEndpoint)
	r.DELETE("/delete-image", deleteEndpoint)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.Run() // listen and serve on 0.0.0.0:8080
}

// @Summary Upload image to s3
// @Produce json
// @Success 200
// @Router /upload-image [post]
// @Param file formData file true "file"
func uploadEndpoint(c *gin.Context) {
	// set uploader
	uploader = NewUploader()

	res := upload(c)
	c.JSON(200, gin.H{
		"message": res,
	})
}

// @Summary Update image from s3
// @Produce json
// @Success 200
// @Router /update-image [put]
// @Param file formData file true "file"
func updateEndpoint(c *gin.Context) {
	// set uploader
	update(c)
	c.JSON(200, gin.H{
		"message": "updated",
	})
}

// @Summary Update image from s3
// @Produce json
// @Success 200
// @Router /delete-image [delete]
// @Param key query string true "key"
// @Param bucket query string true "bucket"
func deleteEndpoint(c *gin.Context) {
	key := c.Query("key")
	bucket := c.Query("bucket")
	deleteObject(key, bucket)

	c.JSON(200, gin.H{
		"message": "deleted",
	})
}

// @Summary Fetch image from cloudfront
// @Produce json
// @Success 200
// @Router /fetch-from-cloud-front [get]
// @Param key query string true "key"
func cloudFrontFetchEndpoint(c *gin.Context) {
	key := c.Query("key")

	cloudFrontDomain := GetEnvWithKey("CLOUD_FRONT_DOMAIN")
	url := fmt.Sprintf("https://%s/%s", cloudFrontDomain, key)

	c.JSON(200, gin.H{
		"url": url,
	})
}

// GetEnvWithKey : get env value
func GetEnvWithKey(key string) string {
	return os.Getenv(key)
}
func LoadEnv() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file")
		os.Exit(1)
	}
}

func NewUploader() *s3manager.Uploader {
	s3Config := &aws.Config{
		Region:      aws.String("us-east-1"),
		Credentials: credentials.NewStaticCredentials(GetEnvWithKey("AWS_ACCESS_KEY_ID"), GetEnvWithKey("AWS_SECRET_ACCESS_KEY"), ""),
	}

	s3Session := session.New(s3Config)
	uploader := s3manager.NewUploader(s3Session)
	return uploader
}

func update(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("Error: %s", err.Error()))
		return
	}

	// Open file
	f, err := file.Open()
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Error: %s", err.Error()))
		return
	}
	defer f.Close()

	// Read file content into buffer
	buffer := make([]byte, file.Size)
	_, err = f.Read(buffer)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Error: %s", err.Error()))
		return
	}

	s3Config := &aws.Config{
		Region:      aws.String("us-east-1"),
		Credentials: credentials.NewStaticCredentials(GetEnvWithKey("AWS_ACCESS_KEY_ID"), GetEnvWithKey("AWS_SECRET_ACCESS_KEY"), ""),
	}

	sess := session.Must(session.NewSession(s3Config))
	svc := s3.New(sess)

	// Upload the updated image file to S3
	_, err = svc.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(GetEnvWithKey("AWS_S3_BUCKET_NAME")),
		Key:         aws.String("test.png"),
		Body:        bytes.NewReader(buffer),
		ContentType: aws.String(file.Header.Get("Content-Type")),
		ACL:         aws.String("public-read"), // set ACL to allow public read access
	})

	if err != nil {
		fmt.Println("Error updating object:", err)
		return
	}

	// invalid from cloudfront
	cloudfrontSession := cloudfront.New(sess)

	distributionID := GetEnvWithKey("CLOD_FRONT_DIST_ID")
	objectKey := "test.png"

	invalidationRequest := &cloudfront.CreateInvalidationInput{
		DistributionId: aws.String(distributionID),
		InvalidationBatch: &cloudfront.InvalidationBatch{
			CallerReference: aws.String(fmt.Sprintf("%d", time.Now().UnixNano())),
			Paths: &cloudfront.Paths{
				Items:    aws.StringSlice([]string{"/" + objectKey}),
				Quantity: aws.Int64(1),
			},
		},
	}

	result, err := cloudfrontSession.CreateInvalidationWithContext(context.Background(), invalidationRequest)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(result)
	fmt.Println("Object updated successfully!")
}

func upload(c *gin.Context) *s3manager.UploadOutput {
	// get file from request

	file, err := c.FormFile("file")
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("Error: %s", err.Error()))
		return nil
	}

	// Open file
	f, err := file.Open()
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Error: %s", err.Error()))
		return nil
	}
	defer f.Close()

	// Read file content into buffer
	buffer := make([]byte, file.Size)
	_, err = f.Read(buffer)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Error: %s", err.Error()))
		return nil
	}

	upInput := &s3manager.UploadInput{
		Bucket:      aws.String(GetEnvWithKey("AWS_S3_BUCKET_NAME")),
		Key:         aws.String(file.Filename), // file's name
		Body:        bytes.NewReader(buffer),   // file's content
		ContentType: aws.String(file.Header.Get("Content-Type")),
		ACL:         aws.String("public-read"), // set ACL to allow public read access
	}
	res, err := uploader.UploadWithContext(context.Background(), upInput)
	log.Printf("res %+v\n", res)
	log.Printf("err %+v\n", err)

	return res
}

// delete object
func deleteObject(key string, bucket string) {
	s3Config := &aws.Config{
		Region:      aws.String("us-east-1"),
		Credentials: credentials.NewStaticCredentials(GetEnvWithKey("AWS_ACCESS_KEY_ID"), GetEnvWithKey("AWS_SECRET_ACCESS_KEY"), ""),
	}

	s3Session := session.New(s3Config)
	svc := s3.New(s3Session)

	_, err := svc.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Successfully deleted object %s from bucket %s", key, bucket)
	return
}
