package utils

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func R2Session() *s3.S3 {
	start := time.Now()
	fmt.Println("[R2Session] ➜ Initializing Cloudflare R2 session...")

	// Check for missing env vars and warn only if they are empty
	requiredEnvVars := []string{"R2_REGION", "R2_ENDPOINT", "R2_ACCESS_KEY_ID", "R2_SECRET_ACCESS_KEY", "R2_BUCKET", "R2_PUBLIC_BASE"}
	for _, key := range requiredEnvVars {
		if os.Getenv(key) == "" {
			fmt.Printf("[R2Session] ⚠️  Environment variable %s is missing or empty\n", key)
		}
	}

	sess, err := session.NewSession(&aws.Config{
		Credentials: credentials.NewStaticCredentials(
			os.Getenv("R2_ACCESS_KEY_ID"),
			os.Getenv("R2_SECRET_ACCESS_KEY"),
			"",
		),
		Endpoint:         aws.String(os.Getenv("R2_ENDPOINT")),
		Region:           aws.String(os.Getenv("R2_REGION")),
		S3ForcePathStyle: aws.Bool(true),
	})

	if err != nil {
		fmt.Println("[R2Session] ❌ Failed to create session:", err)
		panic(err)
	}

	fmt.Println("[R2Session] ✅ R2 session initialized in", time.Since(start))
	return s3.New(sess)
}

// UploadToR2 uploads byte slice data (for backward compatibility)
func UploadToR2(key string, data []byte) (string, error) {
	fmt.Printf("[UploadToR2] ➜ Uploading to R2: key = %s (%d bytes)\n", key, len(data))

	svc := R2Session()

	_, err := svc.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(os.Getenv("R2_BUCKET")),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ACL:         aws.String("public-read"),
		ContentType: aws.String("application/pdf"),
	})
	if err != nil {
		fmt.Println("[UploadToR2] ❌ Upload failed:", err)
		return "", err
	}

	publicBase := os.Getenv("R2_PUBLIC_BASE")
	url := fmt.Sprintf("%s/%s", publicBase, key)
	fmt.Println("[UploadToR2] ✅ Upload successful. File available at:", url)

	return url, nil
}

func UploadStreamToR2(key string, reader io.ReadSeeker) (string, error) {
	fmt.Printf("[UploadStreamToR2] ➜ Streaming upload to R2: key = %s\n", key)

	svc := R2Session()

	_, err := svc.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(os.Getenv("R2_BUCKET")),
		Key:         aws.String(key),
		Body:        reader,
		ACL:         aws.String("public-read"),
		ContentType: aws.String("application/pdf"),
	})
	if err != nil {
		fmt.Println("[UploadStreamToR2] ❌ Streaming upload failed:", err)
		return "", err
	}

	publicBase := os.Getenv("R2_PUBLIC_BASE")
	url := fmt.Sprintf("%s/%s", publicBase, key)
	fmt.Println("[UploadStreamToR2] ✅ Streaming upload successful. File available at:", url)

	return url, nil
}
