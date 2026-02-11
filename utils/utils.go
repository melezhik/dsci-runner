package utils

import (
	"fmt"
	"log"
	"os"
)

func SparkyDbFile() string {

	hdir, _ := os.UserHomeDir()

	return fmt.Sprintf("%s/.dsci/.sparky/projects/db.sqlite3", hdir)

}

func DsciAdminTokenFile() string {

	hdir, _ := os.UserHomeDir()

	return fmt.Sprintf("%s/.dsci.admin.token", hdir)

}

func DsciConfigFile() string {

	hdir, _ := os.UserHomeDir()

	return fmt.Sprintf("%s/.dsci.toml", hdir)

}

func SparkyReportsDir() string {

	hdir, _ := os.UserHomeDir()

	return fmt.Sprintf("%s/.dsci/.sparky/projects/.reports", hdir)

}

func SparkyProjectDir(p string) string {

	hdir, _ := os.UserHomeDir()

	return fmt.Sprintf("%s/.dsci/.sparky/projects/%s", hdir, p)

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
	return fmt.Sprintf("%s/.dsci/.sparky/.cache/%s", hdir, job_id)
}

func SparkyCacheDirDocker(job_id string) string {
	return fmt.Sprintf("/home/worker/.sparky/.cache/%s", job_id)
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
	return fmt.Sprintf("%s/.dsci/.sparky/work/%s/.states", hdir, p)
}
