package utils

import (
	"fmt"
	"log"
	"os"
)

func SessionCacheRootDir() string {

	hdir, _ := os.UserHomeDir()

	return fmt.Sprintf("%s/.dsci/sessions", hdir)

}

func GitCacheRootDir() string {

	hdir, _ := os.UserHomeDir()

	return fmt.Sprintf("%s/.dsci/gitcache", hdir)

}

func GitCloneCacheRootDir() string {

  hdir, _ := os.UserHomeDir()

  return fmt.Sprintf("%s/.dsci/gitclonecache", hdir)

}

func DsciRootDir() string {

	hdir, _ := os.UserHomeDir()

	return fmt.Sprintf("%s/.dsci", hdir)

}

func SparkyDbFile() string {

	hdir, _ := os.UserHomeDir()

	return fmt.Sprintf("%s/.dsci/.sparky/projects/db.sqlite3", hdir)

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
		log.Fatalf("Error creating directory: %s", err)
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
		log.Fatalf("Error creating directory: %s", err)
	}

	return dir
}

func SparkyCacheDir(job_id string) string {
	hdir, _ := os.UserHomeDir()
	return fmt.Sprintf("%s/.dsci/.sparky/.cache/%s", hdir, job_id)
}

func SparkyCacheDirDocker(job_id string, cr string) string {
  if cr == "podman" {
	  return fmt.Sprintf("/root/.sparky/.cache/%s", job_id)
  } else {
	  return fmt.Sprintf("/home/worker/.sparky/.cache/%s", job_id)
  }
}

func CreateSparkyCacheDir(job_id string) string {

	dir := SparkyCacheDir(job_id)

	err := os.MkdirAll(dir, 0755)

	if err != nil {
		log.Fatalf("Error creating directory: %s", err)
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
		log.Fatalf("Error creating directory: %s", err)
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
		log.Fatalf("Error creating directory: %s", err)
	}

	return dir
}

func ProjectStateDir(p string) string {
	hdir, _ := os.UserHomeDir()
	return fmt.Sprintf("%s/.dsci/.sparky/work/%s/.states", hdir, p)
}
