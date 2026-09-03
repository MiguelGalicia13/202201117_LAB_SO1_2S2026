package main

import (
	"path/filepath"
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
func getImages()([]Image , []Image){
	var highImages []Image
	var lowImages []Image
	var goClient Image
	var alpineHigh Image
	var alpineLow Image
	goClient.Name = "go-client"
	goClient.Command = "docker run -d roldyoran/go-client"
	goClient.highusage = true
	highImages = append(highImages, goClient)
	alpineHigh.Name = "alpine"
	alpineHigh.Command = `docker run -d alpine sh -c "while true; do echo '2^20' | bc > /dev/null; sleep 2; done"`
	alpineHigh.highusage = true
	highImages = append(highImages, alpineHigh)
	alpineLow.Name = "alpine"
	alpineLow.Command = "docker run -d alpine sleep 240"
	alpineLow.highusage = false
	lowImages = append(lowImages, alpineLow)
	return highImages, lowImages
}
var ctx = context.Background();

func handleSignals(highUsage *queue.Queue, lowUsage *queue.Queue) {
        sigChan := make(chan os.Signal, 1)
        signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
        sig := <-sigChan
        log.Printf("\n[LOG] STOPPOING NOW: %s. \n", sig)
        highUsage.StopAll()
        lowUsage.StopAll()
        killCronjob()
        cmd := exec.Command("sh", "-c", "docker rm $(docker ps -q -a)")
        out, err:= cmd.Output()
        if err != nil{
        	log.Printf("[ERROR] CONTAINERS COULDNT BE STOPPED %s, %s", err.Error(), out)
         	os.Exit(1)
        }
        log.Println("[LOG] CONTAINERS HAD BEEN STOPPED")
        os.Exit(0)
    }

func main(){
	if len(os.Args) > 1 && os.Args[1] == "add-container" {
		highImager, lowImager :=getImages()
		for i:=0;i<2;i++{
			n := rand.Intn(len(highImager))
			cmd := exec.Command("sh", "-c", highImager[n].Command)
			cmd.Run()
			log.Println("[LOG] HIGH USAGE CONTAINER CREATED")
		}
		for i:=0;i<3;i++{
			n := rand.Intn(len(lowImager))
			cmd := exec.Command("sh", "-c", lowImager[n].Command)
			cmd.Run()
			log.Println("[LOG] LOW USAGE CONTAINER CREATED")
		}
		return
	}
	startModule()
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
		fmt.Sprintf(`(crontab -l 2>/dev/null | grep -v "add-container"; echo "%s" )| crontab -`, cronCommand),
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
        log.Println("[ERROR] COULDNT REMOVE CRONJOB, PLEASE REMOVE IT BY HAND ⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️:", err)                                                                           
        return                                                                                                                                
    }
    log.Println("[LOG] CRONJOB STOPPED")
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
func startModule()error{
	//verify is the modeule up
	if _, err :=os.Stat("/proc/continfo_pr2_so1_202201117"); err==nil{
		log.Println("[LOG] KERNEL MODULE IS ALREADY UP")
		return nil
	}
	execPath, _ := os.Executable()
	kernelPath := filepath.Join(filepath.Dir(execPath),"..","kernel")
	log.Printf("[LOG] COMPILING FROM: %s \n", kernelPath)
	cmd := exec.Command("make", "all")
	cmd.Dir = kernelPath
	out, err := cmd.Output()
	if err!=nil{
		log.Printf("[ERROR] KERNEL COULD NOT BE COMPILED %s ,%d\n",err.Error(),out)
		return err
	}
	log.Println("[LOG] COMPILING COMPLEATED")
	log.Printf("[LOG] LOADING COMPILING FROM: %s \n", kernelPath)
	cmd = exec.Command("sh", "-c", "sudo insmod sysinfo.ko")
	cmd.Dir = kernelPath
	out, err = cmd.Output()
	if err!=nil{
		log.Printf("[ERROR] KERNEL COULD NOT BE LOADED %s, %s\n",err.Error(),out)
		return err
	}
	//clean files created - not neeeded for execution
	cmd = exec.Command("make", "clean")
	cmd.Dir = kernelPath
	_, err = cmd.Output()
	if err!=nil{
		log.Printf("[ERROR] KERNEL COULD NOT BE CLEANED %s\n",err.Error())
		return err
	}
	log.Println("[LOG] KERNEL MODULE UP AND RUNNING")
	return nil
}