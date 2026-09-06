package ebpf

import (
	"bytes"
	"encoding/binary"
	"errors"
	"log"


	cilium "github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"

	"golang.org/x/sys/unix"
)

type KillEvent struct{
	PidEmisor uint32
	PidReciever int32
	Sig int32
}

type Manager struct{
	coll *cilium.Collection
	tpLink link.Link
	reader *ringbuf.Reader
}
func StartKillTrack(objectpath string, onKillConfirmed func(KillEvent))(*Manager, error){
	err := unix.Setrlimit(unix.RLIMIT_MEMLOCK, &unix.Rlimit{
		Cur: unix.RLIM_INFINITY,
		Max: unix.RLIM_INFINITY,
	})
	if err !=nil{
		log.Printf("[ERROR] RMLIT_MEMLOCK WANS ADJUSTED: %v\n",err)
	}
	spec, err := cilium.LoadCollectionSpec(objectpath)
	if err != nil{
		log.Printf("[ERROR] ERROR WHILE LOADING SPEC eBPF: %v\n",err)
		return nil,err
	}
	coll, err := cilium.NewCollection(spec)
	if err != nil{
		log.Printf("[ERROR] ERROR WHILE CREATING COLLECTION FOR eBPF: %v\n",err)
		return nil, err
	}
	prog := coll.Programs["detect_kill"]
	if prog == nil{
		coll.Close()
		log.Println("[ERROR] eBPF PROGRAM NOT FOUND")
		return nil, errors.New("ebpf not found: ")
	}
	tp, err := link.Tracepoint("syscalls", "sys_enter_kill", prog,nil)
	if err != nil{
		coll.Close()
		log.Printf("[ERROR] ERROR ADJUSTING TRACEPOINT: %v",err)
		return nil, err
	}
	events := coll.Maps["events"]
	rd, err :=ringbuf.NewReader(events)
	if err !=nil{
		tp.Close()
		rd.Close()
		log.Printf("[ERROR] ERROR WHILE OPENING RINBUG: %v",err)
		return nil, err
	}
	pm := &Manager{
		coll: coll,
		tpLink: tp,
		reader: rd,
	}
	go func(){
		for {
			record, err := rd.Read()
			if err!=nil{
				if errors.Is(err, ringbuf.ErrClosed){
					return 
				}
				log.Printf("[eBPF] ERROR READING RINGBUFFFER: %v\n",err)
				continue
			}
			var ev KillEvent
			err = binary.Read(bytes.NewBuffer(record.RawSample),binary.LittleEndian, &ev)
			if err != nil{
				log.Printf("[EBPF] ERROR WHILE PARSING EVENT: %v\n", err)
				continue
			}
			onKillConfirmed(ev)
		}
	}()
	return pm,nil
}

func (pm *Manager) Close() {
	if pm.reader != nil {
		pm.reader.Close()
	}
	if pm.tpLink != nil {
		pm.tpLink.Close()
	}
	if pm.coll != nil {
		pm.coll.Close()
	}
}