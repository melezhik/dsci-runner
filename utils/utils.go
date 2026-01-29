package utils

import (
	"fmt"
	"log"
	"os"
)

func SparkyProjectDir(p string) string {

	hdir, _ := os.UserHomeDir()

	return fmt.Sprintf("%s/.sparky/projects/%s", hdir, p)

}


func CreateSparkyProjectDir(p string) string {

	dir := SparkyProjectDir(p)

	err := os.MkdirAll(dir, 0755)

	if err != nil {
		log.Fatalf("Error creating directory:", err)
	}

	return dir
}

func SparkyTriggersDir(p string) string {
	dir := SparkyProjectDir(p)
	return fmt.Sprintf("%s/.triggers", dir)
}

func CreateSparkyTriggersDir(p string) string {

	dir := SparkyTriggersDir(p)

	err := os.MkdirAll(dir, 0755)

	if err != nil {
		log.Fatalf("Error creating directory:", err)
	}

	return dir
}

func SparkyCacheDir(job_id string) string {
	hdir, _ := os.UserHomeDir()
	return fmt.Sprintf("%s/.sparky/.cache/%s", hdir, job_id)
}

func CreateSparkyCacheDir(job_id string) string {

	dir := SparkyCacheDir(job_id)

	err := os.MkdirAll(dir, 0755)

	if err != nil {
		log.Fatalf("Error creating directory:", err)
	}

	return dir
}

func SparkyFilesDir(p string) string {
	dir := SparkyProjectDir(p)
	return fmt.Sprintf("%s/.files", dir)
}

func CreateSparkyFilesDir(p string) string {

	dir := SparkyFilesDir(p)

	err := os.MkdirAll(dir, 0755)

	if err != nil {
		log.Fatalf("Error creating directory:", err)
	}

	return dir
}


func SparkyStashDir(p string) string {
	dir := SparkyProjectDir(p)
	return fmt.Sprintf("%s/.stash", dir)
}

func CreateSparkyStashDir(p string) string {

	dir := SparkyStashDir(p)

	err := os.MkdirAll(dir, 0755)

	if err != nil {
		log.Fatalf("Error creating directory:", err)
	}

	return dir
}


func ProjectStateDir(p string) string {
	hdir, _ := os.UserHomeDir()
	return fmt.Sprintf("%s/.sparky/work/%s/.states", hdir, p )
}
