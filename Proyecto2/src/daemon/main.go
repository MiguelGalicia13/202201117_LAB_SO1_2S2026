package main

import (
	"context"
	"daemon/queue"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)
type Image struct{
	Name string
	Command string
	highusage bool
}
type Process struct{
	PID int `json:"PID"`
	Name string `json:"Name"`
	CmdLine string `json:"Cmdline"`
	Vsz float64 `json:"vsz"`
	Rss float64 `json:"rss"`
	MemoryUsage float64 `json:"Memory_Usage"`
	CpuUsage float64 `json:"CPU_Usage"`
}
type Response struct{
	Total_RAM int `json:"Totalram"`
	FreeRAM int `json:"Freeram"`
	UsedRAM int `json:"Usedram"`
	Procs int `json:"Procs"`
	Process []Process `json:"Processes"`
}

func parseResponse(response string)(Response){
	var resp Response
	err := json.Unmarshal([]byte(response), &resp)
	if err != nil{
		log.Println(err)
	}
	return resp
}
func getImages()([]Image){
	var images []Image
	var goClient Image
	var alpineHigh Image
	var alpineLow Image
	goClient.Name = "go-client"
	goClient.Command = "docker run -d roldyoran/go-client"
	goClient.highusage = true
	images = append(images, goClient)
	alpineHigh.Name = "alpine"
	alpineHigh.Command = `docker run -d alpine sh -c "while true; do echo '2^20' | bc > /dev/null; sleep 2; done"`
	alpineHigh.highusage = true
	images = append(images, alpineHigh)
	alpineLow.Name = "alpine"
	alpineLow.Command = "docker run -d alpine sleep 240"
	alpineLow.highusage = false
	images = append(images, alpineLow)
	return images
}
var ctx = context.Background();

func handleSignals(highUsage *queue.Queue, lowUsage *queue.Queue) {
        sigChan := make(chan os.Signal, 1)
        signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
        sig := <-sigChan
        log.Printf("\n[LOG] STOPPOING NOW: %s. \n", sig)
        highUsage.StopAll()
        lowUsage.StopAll()
        cmd := exec.Command("sh", "-c", "docker rm $(docker ps -q -a)")
        _, err:= cmd.Output()
        if err != nil{
        	log.Println("[ERROR] CONTAINERS COULDNT BE STOPPED")
         	os.Exit(1)
        }
        log.Println("[LOG] CONTAINERS HAD BEEN STOPPED")
        os.Exit(0)
    }

func main(){
	imager :=getImages()
	if len(os.Args) > 1 && os.Args[1] == "add-container" {
		for i:=0;i<5;i++{
			n := rand.Intn(len(imager))
			cmd := exec.Command("sh", "-c", imager[n].Command)
			_, err := cmd.Output()
			if err != nil {
				log.Println("[ERROR] CONTAINTER COULDNT BE CREATED ", err)
				return
			}
			log.Println("[LOG] CONTAINER CREATED")
		}
		return
	}
	var highUsage = queue.NewQueue(2)
	var lowUsage = queue.NewQueue(3)
	//capturar crlt + c
	go handleSignals(highUsage, lowUsage)
	createCronjob()
	for {
		time.Sleep(20 * time.Second)
	}
    }
    
    func generateContainer(queueHigh *queue.Queue, queueLow *queue.Queue, imgs []Image) {
	command := "cat /proc/continfo_pr2_so1_202201117"
	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.Output()
	if err != nil {
		log.Println(err)
		return
	}
	resp := parseResponse(string(output))
	var container queue.Container
	lastProces := resp.Process[resp.Procs-1]
	container.PID = lastProces.PID
	container.Name = lastProces.Name
	container.CMDL = lastProces.CmdLine
	container.CPU = lastProces.CpuUsage
	container.Memory = lastProces.MemoryUsage
	container.VSZ = lastProces.Vsz
	container.RSS = lastProces.Rss
	container.Status = "running"
	/*if image.highusage {
		queueHigh.Enqueue(container)
	} else {
		queueLow.Enqueue(container)
	}*/
    }
    
    func createCronjob(){
	executable, _ := os.Executable()
	cronCommand := fmt.Sprintf("*/2 * * * * %s add-container", executable)
    
	out, err := exec.Command("bash", "-c", 
		fmt.Sprintf(`(crontab -l 2>dev/null; echo "%s" )| crontab -`, cronCommand),
	).Output()
	if err != nil{
		log.Printf("[LOG] ERROR CREATING CRONJOB: %v, OUTPUT: %s \n", err, out)
	}else{
		log.Println("[LOG] CRONJOB CREATED SUCCESFULY")
	}
}