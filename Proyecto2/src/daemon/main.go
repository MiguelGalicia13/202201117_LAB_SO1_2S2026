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
        /*cmd := exec.Command("sh", "-c", "docker rm $(docker ps -q -a)")
        _, err:= cmd.Output()
        if err != nil{
        	log.Println("[ERROR] CONTAINERS COULDNT BE STOPPED")
         	os.Exit(1)
        }*/
        killCronjob()
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
		resp, err := readKernelModule()
		if err !=nil{
			log.Println("[ERROR] COULDNT READ /proc: ",err.Error())
			continue
		}
		for _,p := range resp.Process{
			//for each of containers
			// skip grafana n Valkey
			if  p.Name == "grafana" || p.Name=="valkey-server"{
				continue
			}
			container := queue.Container{
				PID: p.PID,
				Name: p.Name,
				CMDL: p.CmdLine,
				Memory: p.MemoryUsage,
				VSZ: p.Vsz,
				RSS: p.Rss,
				Status: "running",
			}
			if isHighUsage(p){
				if !highUsage.Contains(p.PID){
					log.Printf("[DAEMON] NEW HIGH USAGE CONTAINER DETECTED (PID: %d)...\n", p.PID)
					highUsage.Enqueue(container)
				}
			}else{
				if !lowUsage.Contains(p.PID){
					log.Printf("[DAEMON] NEW LOW USAGE CONTAINER DETECTED (PID: %d)...\n", p.PID)
					lowUsage.Enqueue(container)
				}
			}
		}
		//balance containers
		for highUsage.Size() < highUsage.MaxSize(){
			log.Println("[DAEMON] HIGH USAGE CONTAINER MISSING. CREATING A NEW ONE")
			cmd := exec.Command("sh", "-c", "docker run -d roldyoran/go-client")
			cmd.Run()
			break
		}
		for lowUsage.Size() < lowUsage.MaxSize(){
			log.Println("[DAEMON] LOW USAGE CONTAINER MISSING. CREATING A NEW ONE")
			  cmd := exec.Command("sh", "-c", "docker run -d alpine sleep 240")
	        cmd.Run()
	        break
		}
		log.Printf("[DAEMON] DATA: %d PROCESS DETECTED | RAM USED: %d MB\n", resp.Procs, resp.UsedRAM)
	}
}

func readKernelModule()(*Response, error) {
	command := "cat /proc/continfo_pr2_so1_202201117"
	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.Output()
	if err != nil {
		return nil,err
	}
	resp := parseResponse(string(output))
	return &resp, nil
    }
    
func createCronjob(){
	executable, _ := os.Executable()
	cronCommand := fmt.Sprintf("*/2 * * * * %s add-container", executable)
    
	out, err := exec.Command("bash", "-c", 
		fmt.Sprintf(`(crontab -l 2>/dev/null; echo "%s" )| crontab -`, cronCommand),
	).Output()
	if err != nil{
		log.Printf("[LOG] ERROR CREATING CRONJOB: %v, OUTPUT: %s \n", err, out)
	}else{
		log.Println("[LOG] CRONJOB CREATED SUCCESFULY")
	}
}

func killCronjob(){
	cmd := exec.Command("bash", "-c", "crontab -l 2>/dev/null | grep -v 'add-container' | crontab -")
    err := cmd.Run()
    if err != nil {
        log.Println("[ERROR] No se pudo eliminar el cronjob:", err)                                                                           
        return                                                                                                                                
    }
    log.Println("[LOG] CRONJOB ELIMINADO EXITOSAMENTE DEL SISTEMA")
}
func isHighUsage(p Process)bool{
	if p.Name == "client" || p.Name =="sh"{
		return true
	}
	if p.CpuUsage > 10.0 || p.MemoryUsage > 5.0{
		return true
	}
	return false
}