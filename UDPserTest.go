package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const Nconveer = 10
const TTLsec = 100
const HOMEPLACE = "data/"

type kttt struct {
	updlen    int
	udppacket []byte
	addr      *net.UDPAddr
	MutP      sync.Mutex
	flag      int
}

func UDPread(conn *net.UDPConn, packets *[Nconveer]kttt, ch chan int) {
	ex := 0
	var j, f int
	var err error

	for ex == 0 {
		f = 0
		for j = 0; j < Nconveer && f == 0; j++ {
			packets[j].MutP.Lock()
			if packets[j].flag == 0 {
				packets[j].updlen, packets[j].addr, err = conn.ReadFromUDP(packets[j].udppacket)
				if err == nil {
					if packets[j].updlen > 0 {
						packets[j].flag = 1
						//fmt.Println("[", packets[j].updlen, "]")
						ch <- j
						f = 1
					}
				} else {
					fmt.Println("[", packets[j].updlen, "]")
					fmt.Println(err)
					f = 1
				}
			}
			packets[j].MutP.Unlock()
		}
		if f == 0 {
			packets[Nconveer-1].MutP.Lock()
			packets[Nconveer-1].updlen, packets[Nconveer-1].addr, err = conn.ReadFromUDP(packets[Nconveer-1].udppacket)
			if err == nil {
				if packets[Nconveer-1].updlen > 0 {
					packets[Nconveer-1].flag = 1
					//fmt.Println("[", packets[Nconveer-1].updlen, "]")
					ch <- Nconveer - 1
				} else {
					fmt.Println("[", packets[Nconveer-1].updlen, "]")
					fmt.Println(err)
				}
			}
			packets[Nconveer-1].MutP.Unlock()
		}
		//SpeedDelay(0, 10)
		time.Sleep(10 * time.Microsecond)
	}
	fmt.Println("Exit UDP reader")
}

var tokens [10]string

func sendResponse(conn *net.UDPConn, msg string, addr *net.UDPAddr) {
	_, err := conn.WriteToUDP([]byte(msg), addr)
	if err != nil {
		fmt.Printf("Couldn't send response %v", err)
	}
}

type nIds struct {
	flag   int
	offset uint64
	idlen  int
}
type mframes struct {
	num   int
	flag  uint8
	crc   uint32
	myIds []nIds
}
type stmm struct {
	UDPaddr   net.UDPAddr
	addr      []uint8
	TimeStamp time.Time
	token     string
	bufR      []uint8
	ch        chan int
	chexit    chan int
	framelen  int
	frames    []mframes
	idLen     int
	pingcnt   int
	curfile   string
	flag      int
}

var stateMachine []stmm

func myStateMachine(conn *net.UDPConn, ind int) {
	var i uint64
	var j, k, fl int
	var nid nIds
	var moffset int64
	var mbuf []uint8
	var cstr string
	var astr string
	var bstr string
	var totallen int
	var ddur time.Duration
	var Filename string
	var Filelen uint64
	var FileCRC uint64
	var Nframes int
	var Nids int
	var Plen int
	var frameUn mframes
	var f *os.File
	var err error
	// стейт машина запускаемая на каждый сокет
	fmt.Println("Start state:", ind)

	ex := 0
	fl = 0
	stateMachine[ind].pingcnt = 0
	stateMachine[ind].flag = 1

	mystate := -1
	mbuf = stateMachine[ind].bufR[:]
	go sendResponse(conn, string(mbuf), &stateMachine[ind].UDPaddr)
	for ex == 0 {
		select {
		case <-stateMachine[ind].ch:

			//fmt.Println("+")
			stateMachine[ind].TimeStamp = time.Now()
			mbuf = stateMachine[ind].bufR[:]
			totallen = len(stateMachine[ind].bufR)
			if totallen < 100 {
				cstr = string(stateMachine[ind].bufR[:])
			} else {
				cstr = string(stateMachine[ind].bufR[:25])
			}
			fmt.Println("##", mystate, fl, ">>", cstr)
			if len(mbuf) > 1 {
				if string(mbuf[:2]) == "w/" {
					if mystate == 3 {
						//fmt.Println("жир")
						mumu := ""
						for j = 2; j < 30 && mbuf[j] != '/'; j++ {
							mumu += string(mbuf[j])
						}
						//fmt.Println(mumu)
						uuo, _ := strconv.Atoi(mumu)
						tumu := string(mbuf[2:uuo])
						aupar := strings.Split(tumu, "/")
						fmt.Println(aupar)

						moffset, _ = strconv.ParseInt(aupar[1], 0, 64)
						m_fr, _ := strconv.Atoi(aupar[2])
						m_id, _ := strconv.Atoi(aupar[3])
						m_len, _ := strconv.Atoi(aupar[4])
						fmt.Println("Fr:", m_fr, " IdN:", m_id)
						stateMachine[ind].frames[m_fr].myIds[m_id].flag = 1
						fmt.Println(tumu)
						f.Seek(moffset, 0)
						n, e := f.Write(mbuf[uuo : uuo+m_len])
						if e != nil {
							fmt.Println(e)
						} else {
							fmt.Println("bytes:", n)
						}
					} else {
						fmt.Println("неверное состояние:", mystate)
					}
				}
			}
			//_, _ = conn.WriteToUDP().Write([]uint8("..."))
			//go sendResponse(conn, string(mbuf), &stateMachine[ind].UDPaddr)
			switch {
			case mystate == -1:
				{
					if string(mbuf[:1]) == "." {
						go sendResponse(conn, string("ping"), &stateMachine[ind].UDPaddr)

					}
					mystate = 0
				}
			case mystate == 0:
				if len(mbuf) > 2 {
					if string(mbuf[:2]) == "a/" && fl == 0 {
						fl = 1
						fmt.Println("sended header")
						cstr = string(mbuf[:])
						ustr := strings.Split(cstr, "/")
						if len(ustr) > 6 {
							Filename = ustr[1]
							Filelen, _ = strconv.ParseUint(ustr[2], 0, 64)
							FileCRC, _ = strconv.ParseUint(ustr[3], 0, 64)
							Nframes, _ = strconv.Atoi(ustr[4])
							Nids, _ = strconv.Atoi(ustr[5])
							Plen, _ = strconv.Atoi(ustr[6])
							fmt.Println(FileCRC, "FR:", Nframes, "ID", Nids)
							ku := uint64(Filelen)
							stateMachine[ind].frames = make([]mframes, 0)

							for i = 0; i < ku; {
								for j = 0; j < Nframes; j++ {
									frameUn.myIds = make([]nIds, 0)
									for k = 0; k < Nids; k++ {

										nid.flag = 0

										if ku > i+uint64(Plen) {
											nid.idlen = Plen

											nid.offset = i
										} else {
											if i > ku {
												nid.idlen = 0

												nid.offset = ku
											} else {
												nid.idlen = int(ku - i)

												nid.offset = i
											}

										}
										if i+uint64(Plen) < ku {
											i = i + uint64(Plen)
										} else {
											i = ku
										}
										frameUn.myIds = append(frameUn.myIds, nid)
									}
									frameUn.num = j
									stateMachine[ind].frames = append(stateMachine[ind].frames, frameUn)
								}
							}
							/*for j = 0; j < Nframes; j++ {
								fmt.Println("N FRAME:", j)
								for k = 0; k < Nids; k++ {
									fmt.Println("\t", k, ".", stateMachine[ind].frames[j].myIds[k].flag, stateMachine[ind].frames[j].myIds[k].offset, stateMachine[ind].frames[j].myIds[k].idlen)
								}
								fmt.Println("")
							} */

							mystate = 2
						}
						go sendResponse(conn, string("a!"), &stateMachine[ind].UDPaddr)
						fl = 0
					} else {
						if string(mbuf[:3]) == "fin" {
							go sendResponse(conn, "fok", &stateMachine[ind].UDPaddr)
							mystate = -1
							ex = 1
							time.Sleep(100 * time.Millisecond)
							go sendResponse(conn, "fok", &stateMachine[ind].UDPaddr)
						}
					}
				} else {
					if mystate != 3 {
						time.Sleep(100 * time.Millisecond)
						go sendResponse(conn, string("a#"), &stateMachine[ind].UDPaddr)

					}

				}

			case mystate == 1:
			case mystate == 3:
				if len(mbuf) > 1 {
					if string(mbuf[0:2]) == "a/" {
						go sendResponse(conn, string("a!"), &stateMachine[ind].UDPaddr)
					} else {
						if len(mbuf) > 4 {
							if string(mbuf[0:4]) == "pung" {
								cf := 0
								if len(mbuf) > 4 {
									cf, _ = strconv.Atoi(string(mbuf[4:]))
									fmt.Println("p ", cf)
									cstr = ""
									for k = 0; k < cf+1; k++ {
										for j = 0; j < Nids; j++ {
											if stateMachine[ind].frames[k].myIds[j].flag == 0 {
												astr = strconv.Itoa(k)
												bstr = strconv.Itoa(j)
												cstr += astr + "," + bstr + ";"
											}
										}
									}
									if len(cstr) == 0 {
										go sendResponse(conn, string("pang")+strconv.Itoa(cf), &stateMachine[ind].UDPaddr)
									} else {
										go sendResponse(conn, string("ret/")+cstr, &stateMachine[ind].UDPaddr)

									}
								}

							}
						} else {
							if string(mbuf) == "fin" {
								go sendResponse(conn, "fok", &stateMachine[ind].UDPaddr)
								ex = 1
								time.Sleep(100 * time.Millisecond)
								go sendResponse(conn, "fok", &stateMachine[ind].UDPaddr)
							}
						}

					}

				} else {
					ex = 1
				}

			}
		default:
			ddur = time.Since(stateMachine[ind].TimeStamp)
			if int(ddur.Seconds()) > 10 {
				ex = 1
			}
			switch mystate {
			case 0:
				/*				lpp := strconv.Itoa(mystate)
								utt := []uint8(lpp)
								rbuf := mbuf
								fmt.Println(lpp)
								rbuf = append(rbuf, utt...)
								go sendResponse(conn, string(rbuf), &stateMachine[ind].UDPaddr)*/
			case 1:
				if len(mbuf) > 2 {
					if string(mbuf[:2]) == "a/" {
						//a/имя-файла/len/FILEcrc/nframes/nids/idlen
						cstr = string(mbuf[:])
						ustr := strings.Split(cstr, "/")
						if len(ustr) > 6 {
							Filename = ustr[1]
							Filelen, _ = strconv.ParseUint(ustr[2], 0, 64)
							FileCRC, _ = strconv.ParseUint(ustr[3], 0, 64)
							Nframes, _ = strconv.Atoi(ustr[4])
							Nids, _ = strconv.Atoi(ustr[5])
							Plen, _ = strconv.Atoi(ustr[6])
							fmt.Println(FileCRC, "FR:", Nframes, "ID", Nids)
							ku := uint64(Filelen)
							stateMachine[ind].frames = make([]mframes, 0)

							for i = 0; i < ku; {
								for j = 0; j < Nframes; j++ {
									frameUn.myIds = make([]nIds, 0)
									for k = 0; k < Nids; k++ {

										nid.flag = 0

										if ku > i+uint64(Plen) {
											nid.idlen = Plen

											nid.offset = i
										} else {
											if i > ku {
												nid.idlen = 0

												nid.offset = ku
											} else {
												nid.idlen = int(ku - i)

												nid.offset = i
											}

										}
										if i+uint64(Plen) < ku {
											i = i + uint64(Plen)
										} else {
											i = ku
										}
										frameUn.myIds = append(frameUn.myIds, nid)
									}
									frameUn.num = j
									stateMachine[ind].frames = append(stateMachine[ind].frames, frameUn)
								}
							}

							mystate = 2
						}

					}
				}
			case 2:
				f, err = os.Create("./" + HOMEPLACE + Filename)
				if err != nil {
					fmt.Println("Не могу создать файл")
				} else {
					mystate = 3
				}
			case 3:
			case 4:

			}
		}

		time.Sleep(10 * time.Microsecond)
	}

	f.Close()

	//time.Sleep(1 * time.Second)
	stateMachine[ind].flag = 0
	fmt.Println("-------------------------------------exit:", ind)
}
func handleUDPConnection(conn *net.UDPConn) {
	//	var kstm stmm
	var x, i, ind, exu int
	var ch chan int
	var buf []uint8
	var mlen int
	var dname []uint8
	var un stmm
	var name, maddr string
	var UDPaddr net.UDPAddr
	var packets [Nconveer]kttt
	for i := 0; i < Nconveer; i++ {
		packets[i].udppacket = make([]byte, 8024)
		packets[i].flag = 0
	}

	//	var UDPaddr net.UDPAddr
	ch = make(chan int)
	stateMachine = make([]stmm, 0)
	fmt.Println("Init")
	go UDPread(conn, &packets, ch)
	fmt.Println("Start handle")
	ex := 0
	for ex == 0 {
		select {
		case x = <-ch:
			//mt.Println("+")
			name = ""
			for i = 0; i < len(stateMachine); i++ {
				fmt.Println(string(stateMachine[i].addr), stateMachine[i].flag)
			}
			packets[x].MutP.Lock()
			dname = make([]uint8, 0)
			maddr = packets[x].addr.String()
			for i = 0; i < len(maddr); i++ {
				if maddr[i] >= '0' && maddr[i] <= '9' {
					dname = append([]uint8(name), maddr[i])
					name = string(dname)
				}
			}
			packets[x].flag = 0
			mlen = packets[x].updlen
			buf = make([]uint8, mlen)
			//UDPaddr = *packets[x].addr
			copy(buf, packets[x].udppacket)
			UDPaddr = *packets[x].addr
			packets[x].MutP.Unlock()
			fmt.Println("!")
			exu = -1
			for i = 0; i < len(stateMachine) && exu == -1; i++ {
				//fmt.Println(string(stateMachine[i].addr), name)
				if string(stateMachine[i].addr) == name && stateMachine[i].flag == 1 {
					//fmt.Println(i, "---")
					exu = i
					stateMachine[i].bufR = make([]uint8, mlen)
					copy(stateMachine[i].bufR, buf)
					stateMachine[i].TimeStamp = time.Now().UTC()
					stateMachine[i].UDPaddr = UDPaddr
					stateMachine[i].ch <- 1
				}
			}
			if exu == -1 {
				//запуск машины
				for i = 0; i < len(stateMachine) && exu == -1; i++ {
					if stateMachine[i].flag == 0 {
						stateMachine[i].addr = []uint8(name)
						stateMachine[i].bufR = make([]uint8, mlen)
						copy(stateMachine[i].bufR, buf)
						stateMachine[i].TimeStamp = time.Now().UTC()
						stateMachine[i].UDPaddr = UDPaddr
						go myStateMachine(conn, i)
						exu = i
					}
				}
				if exu == -1 {
					un.addr = []uint8(name)
					un.flag = 1
					un.TimeStamp = time.Now().UTC()
					un.UDPaddr = UDPaddr
					un.ch = make(chan int, 0)
					un.bufR = make([]uint8, mlen)
					copy(un.bufR, buf)
					stateMachine = append(stateMachine, un)
					ind = len(stateMachine) - 1
					go myStateMachine(conn, ind)
				}

			}

			//conn.Write(buf[:])
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	fmt.Println("exit handle")
}

func main() {
	//	iniMyWatch()

	hostName := "localhost"
	portNum := "5000"
	service := hostName + ":" + portNum

	udpAddr, err := net.ResolveUDPAddr("udp4", service)
	//	testfunc()
	if err == nil {
		ln, err := net.ListenUDP("udp", udpAddr)

		if err == nil {
			fmt.Println("UDP сервер поднят ,порт: " + portNum)
			for {
				fmt.Println("_")
				handleUDPConnection(ln)
			}
			ln.Close()
		} else {
			fmt.Println(err)
		}

	} else {
		fmt.Println(err)
	}

}
