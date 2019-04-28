package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const Nconveer = 1000
const TTLsec = 100

const DATAPATH = "./data/"
const ARCHPATH = "./arch/"

//---config-------
var pathDir string = "./data"
var hostName string = "localhost"
var portNum string = "3000"
var token string = "123456"

//----------------

type kttt struct {
	updlen    int
	udppacket []byte
	addr      *net.UDPAddr
	MutP      sync.Mutex
	flag      int
}

var packets [Nconveer]kttt

func sendResponse(conn *net.UDPConn, msg string, addr *net.UDPAddr) {
	_, err := conn.WriteToUDP([]byte(msg), addr)
	if err != nil {
		fmt.Printf("Couldn't send response %v", err)
	}
}
func UDPread(conn *net.UDPConn, packets *[Nconveer]kttt, ch chan int) {
	ex := 0
	var j, f int
	var err error
	var pconn net.UDPConn = *conn
	for j = 0; j < Nconveer; j++ {
		packets[j].udppacket = make([]uint8, 8000)
	}
	for ex == 0 {
		f = 0
		for j = 0; j < Nconveer && f == 0; j++ {
			packets[j].MutP.Lock()
			if packets[j].flag == 0 {
				packets[j].updlen, packets[j].addr, err = pconn.ReadFromUDP(packets[j].udppacket)
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
					ex = 1
				}
			}
			packets[j].MutP.Unlock()
		}
		if f == 0 {
			packets[Nconveer-1].MutP.Lock()
			packets[Nconveer-1].updlen, packets[Nconveer-1].addr, err = pconn.ReadFromUDP(packets[Nconveer-1].udppacket)
			if err == nil {
				if packets[Nconveer-1].updlen > 0 {
					packets[Nconveer-1].flag = 1
					//fmt.Println("[", packets[Nconveer-1].updlen, "]")
					ch <- Nconveer - 1
				} else {
					fmt.Println("[", packets[Nconveer-1].updlen, "]")
					fmt.Println(err)
					ex = 1
				}
			}
			packets[Nconveer-1].MutP.Unlock()
		}
		//SpeedDelay(0, 10)
		time.Sleep(10 * time.Microsecond)
	}
}

type nIds struct {
	flag   int
	offset uint64
	idlen  int
}
type mframes struct {
	num   int
	crc   uint32
	myIds []nIds
}

const NFRAMES = 6
const NIDS = 20
const PLEN = 6000

type uuh struct {
	frame int
	id    int
}

func handleUDPConnection(zconn *net.UDPConn, chrez chan int, filesend string, raddr *net.UDPAddr) {
	var nid nIds
	var frameUn mframes
	var frames []mframes
	var zuh []uuh
	var lun uuh
	var i, ku uint64
	var pungcnt, delay int
	var j, k int
	var msg string
	var zstr string
	var rez, plen int
	var ch chan int
	var NFrames int = 0
	var oldmsg string
	var oldmsgcnt int
	var CURENTframe int
	var ddur time.Duration
	var Filelen uint64
	var FileCRC uint64
	var TimeStamp time.Time = time.Now().UTC()
	var data []uint8
	var ee error
	var pconn net.UDPConn = *zconn
	//	var UDPaddr net.UDPAddr = *raddr
	var packets [Nconveer]kttt
	for i := 0; i < Nconveer; i++ {
		packets[i].udppacket = make([]byte, 8024)
		packets[i].flag = 0
	}
	zuh = make([]uuh, 0)
	ch = make(chan int, 100)
	go UDPread(&pconn, &packets, ch)
	rez = 0
	fmt.Println("Init")

	fmt.Println("Start handle")
	ex := 0
	mystate := -1
	for ex == 0 {
		select {
		case x := <-ch:
			//fmt.Println(".")
			TimeStamp = time.Now().UTC()
			mystate = 0
			packets[x].MutP.Lock()
			packets[x].flag = 0
			msg = string(packets[x].udppacket[:packets[x].updlen])
			//			UDPaddr := packets[x].addr
			packets[x].MutP.Unlock()
			fmt.Println(msg, ">>", mystate, "lmsg", len(msg))
			if msg == oldmsg {
				if oldmsgcnt < 200 {
					oldmsgcnt++
				} else {
					ex = 1
					rez = 0
				}
			} else {
				oldmsg = msg
				oldmsgcnt = 0
			}
			switch {
			case strings.Contains(msg, "ret/"):
				zstr = string([]byte(msg)[4:])
				austr := strings.Split(zstr, ";")
				zuh = make([]uuh, 0)
				delay = len(zuh)
				for j = 0; j < len(austr); j++ {
					mvstr := strings.Split(austr[j], ",")
					if len(mvstr) > 1 {
						lun.frame, _ = strconv.Atoi(mvstr[0])
						lun.id, _ = strconv.Atoi(mvstr[1])
						zuh = append(zuh, lun)
					}

				}
				mystate = 5
			case msg == "ping":
				pconn.Write([]uint8("pong"))
				if mystate == -1 {
					mystate = 0
				}
			case msg == "fok":
				mystate = 7
			case strings.Contains(msg, "pang"):
				delay = delay / 2
				if CURENTframe < NFrames {
					if mystate != 2 {
						ass, e1 := strconv.Atoi(string([]byte(msg)[4:]))
						if e1 == nil {
							CURENTframe = ass + 1
						}
						mystate = 2
					}
				} else {
					mystate = 6
				}
			case msg == "a!":
				fmt.Println("Let's start!")
				mystate = 2
			case msg == "a#": //a/имя-файла/len/FILEcrc/nframes/nids/idlen
				mystr := "a/" + filesend + "/"
				MyInfo, eee := os.Stat(DATAPATH + filesend)
				if eee == nil {
					Filelen = uint64(MyInfo.Size())
					fmt.Println("Длина файла:", Filelen)
					mystr += strconv.FormatUint(Filelen, 10) + "/"
					data, ee = ioutil.ReadFile(DATAPATH + filesend)
					if ee == nil {
						FileCRC = 0
						for i = 0; i < uint64(len(data)); i++ {
							FileCRC += uint64(data[i])
						}
						mystr += strconv.FormatUint(FileCRC, 10) + "/"
						//NFrames = NFRAMES
						//plen = int(Filelen/(NFRAMES*NIDS)) + 1
						plen = PLEN
						if uint64(plen) > Filelen {
							NFrames = 1
						} else {
							NFrames = int((Filelen / (uint64(plen) * NIDS)) + 1)
						}

						//plen = int(Filelen / uint64(NIds))
						// a/nframes/idlen/nids
						mystr += strconv.Itoa(NFrames) + "/"
						mystr += strconv.Itoa(NIDS) + "/"
						mystr += strconv.Itoa(plen) + "/"
						ku = uint64(len(data))
						frames = make([]mframes, 0)
						for j = 0; j < NFrames && i < ku; j++ {
							fmt.Println("-f")
							frameUn.myIds = make([]nIds, 0)
							for k = 0; k < NIDS; k++ {
								fmt.Println("\t.")
								nid.flag = 0

								if ku > i+uint64(plen) {
									nid.idlen = plen
									nid.flag = 0
									nid.offset = i
								} else {
									if i > ku {
										nid.idlen = 0
										nid.flag = 0
										nid.offset = ku
									} else {
										nid.idlen = int(ku - i)
										nid.flag = 1
										nid.offset = i
									}

								}
								if i+uint64(plen) < ku {
									i = i + uint64(plen)
								} else {
									i = ku
								}
								frameUn.myIds = append(frameUn.myIds, nid)
							}
							frameUn.num = j
							frames = append(frames, frameUn)
						}

						pconn.Write([]uint8(mystr))
					} else {
						fmt.Println("quit1")
						ex = 1
						rez = 0
					}
				} else {
					fmt.Println("quit2", eee)
					ex = 1
					rez = 0
				}

			case msg == "o#":
				mystate = 5
				//default:
				//	fmt.Println("def:", msg)
				//	mystate = 0
			}
		default:
			ddur = time.Since(TimeStamp)
			if int(ddur.Seconds()) > 4 {
				ex = 1
				rez = 0
			}
			switch mystate {
			case -1:
				pconn.Write([]uint8("."))
				time.Sleep(1000 * time.Millisecond)
				//conn.Write([]uint8("."))
			case 0:
				mystr := "a/" + filesend + "/"
				MyInfo, eee := os.Stat(DATAPATH + filesend)
				//fmt.Print(MyInfo)
				if eee == nil {
					Filelen = uint64(MyInfo.Size())
					//fmt.Println("Длина файла:", Filelen)
					if Filelen > 0 {
						mystr += strconv.FormatUint(Filelen, 10) + "/"
						data, ee = ioutil.ReadFile(DATAPATH + filesend)
						if ee == nil {
							FileCRC = 0
							for i = 0; i < uint64(len(data)); i++ {
								FileCRC += uint64(data[i])
							}
							mystr += strconv.FormatUint(FileCRC, 10) + "/"
							//NFrames = NFRAMES
							//plen = int(Filelen/(NFRAMES*NIDS)) + 1
							plen = 6000
							if uint64(plen) > Filelen {
								NFrames = 1
							} else {
								NFrames = int((Filelen / (uint64(plen) * NIDS)) + 1)
							}
							fmt.Println("NFrames:", NFrames)
							//plen = int(Filelen / uint64(NIds))
							// a/nframes/idlen/nids
							mystr += strconv.Itoa(NFrames) + "/"
							mystr += strconv.Itoa(NIDS) + "/"
							mystr += strconv.Itoa(plen) + "/"
							ku = uint64(len(data))
							frames = make([]mframes, 0)
							i = 0
							fmt.Println("filelen", ku)
							for j = 0; j < NFrames && i < ku; j++ {
								fmt.Println("-f")
								frameUn.myIds = make([]nIds, 0)
								for k = 0; k < NIDS; k++ {
									fmt.Println("\t.")
									nid.flag = 0

									if ku > i+uint64(plen) {
										nid.idlen = plen
										nid.flag = 0
										nid.offset = i
									} else {
										if i > ku {
											nid.idlen = 0
											nid.flag = 0
											nid.offset = ku
										} else {
											nid.idlen = int(ku - i)
											nid.flag = 1
											nid.offset = i
										}

									}
									if i+uint64(plen) < ku {
										i = i + uint64(plen)
									} else {
										i = ku
									}
									frameUn.myIds = append(frameUn.myIds, nid)
								}
								frameUn.num = j
								frames = append(frames, frameUn)
							}

							/*							for i = 0; i < uint64(NFrames); i++ {
														fmt.Println("Frame:", i)
														for k = 0; k < NIDS; k++ {
															fmt.Println(k, "\tID:", k, "flag:", frames[i].myIds[k].flag)
														}
													}*/

							pconn.Write([]uint8(mystr))
						} else {
							fmt.Println("quit1")
							ex = 1
							rez = 0
						}
					} else {
						fmt.Println("Нулевой размер!")
						ex = 1
						rez = 0
					}
				} else {
					fmt.Println("quit2")
					ex = 1
					rez = 0
				}

			case 2: //w/offset/NF/nid/len/data

				//f, err := os.Create("./dat.tmp")
				if CURENTframe < NFrames {
					fmt.Println(CURENTframe)
					//if err == nil {
					//for j = 0; j < len(frames); j++ {

					fmt.Println(NIDS, CURENTframe, frames[CURENTframe].myIds[19].flag)
					for k = 0; k < NIDS && frames[CURENTframe].myIds[k].flag != 1; k++ {
						tmpstr := "/" + strconv.FormatUint(frames[CURENTframe].myIds[k].offset, 10) + "/" + strconv.FormatUint(uint64(CURENTframe), 10) + "/" + strconv.Itoa(int(k)) + "/" + strconv.Itoa(int(frames[CURENTframe].myIds[k].idlen))
						gstr := "w/" + strconv.Itoa(len(tmpstr)+4)
						gstr += tmpstr
						ustr := []uint8(gstr)
						//f.Seek(int64(frames[j].myIds[k].offset), 0)
						mydata := data[int(frames[CURENTframe].myIds[k].offset) : int(frames[CURENTframe].myIds[k].offset)+int(frames[CURENTframe].myIds[k].idlen)]
						ustr = append(ustr, mydata...)
						pconn.Write(ustr)
						//f.Write(mydata)
						for j = 0; j < delay; j++ {
							time.Sleep(100 * time.Millisecond)
						}
					}
					//}
					//f.Close()
					//}
					mystate = 4
				} else {
					mystate = 6
				}

			case 4:
				pconn.Write([]uint8("pung" + strconv.Itoa(CURENTframe)))
				for j = 0; j < int(delay/2); j++ {
					time.Sleep(10 * time.Millisecond)
				}
				if pungcnt < 50 {
					pungcnt++
				} else {
					mystate = 0
				}
			case 5:
				for j = 0; j < len(zuh); j++ {
					tmpstr := "/" + strconv.FormatUint(frames[zuh[j].frame].myIds[zuh[j].id].offset, 10) + "/" + strconv.FormatUint(uint64(zuh[j].frame), 10) + "/" + strconv.Itoa(int(zuh[j].id)) + "/" + strconv.Itoa(int(frames[zuh[j].frame].myIds[zuh[j].id].idlen))
					gstr := "w/" + strconv.Itoa(len(tmpstr)+4)
					gstr += tmpstr
					ustr := []uint8(gstr)
					//f.Seek(int64(frames[j].myIds[k].offset), 0)
					mydata := data[int(frames[zuh[j].frame].myIds[zuh[j].id].offset) : int(frames[zuh[j].frame].myIds[zuh[j].id].offset)+int(frames[zuh[j].frame].myIds[zuh[j].id].idlen)]
					ustr = append(ustr, mydata...)
					pconn.Write(ustr)
					//f.Write(mydata)
					time.Sleep(100 * time.Millisecond)
				}
				mystate = 4
			case 6:
				time.Sleep(500 * time.Millisecond)
				pconn.Write([]uint8("fin"))

			case 7:
				rez = 1
				ex = 1
			}
		}
	}

	fmt.Println("exit handle")
	pconn.Close()
	chrez <- rez
}

/*
func handleUDPConnection(conn *net.UDPConn, chrez chan int, ch chan int, filesend string, raddr *net.UDPAddr) {
	var nid nIds
	var frameUn mframes
	var frames []mframes
	var i uint64
	var j, k int
	var msg string
	var rez, plen int
	//	var ch chan int
	//	var NFrames int
	//	var NIds int
	var Filelen uint64
	var FileCRC uint64
	var data []uint8
	var ee error
	//	var UDPaddr net.UDPAddr = *raddr

	fmt.Println("Init")

	fmt.Println("Start handle")
	ex := 0
	mystate := -1
	for ex == 0 {
		select {
		case x := <-ch:
			fmt.Println(".")
			packets[x].MutP.Lock()
			packets[x].flag = 0
			msg = string(packets[x].udppacket[:packets[x].updlen])
			//			UDPaddr = *packets[x].addr
			packets[x].MutP.Unlock()
			fmt.Println(msg, ">>", mystate, "lmsg", len(msg))
			switch msg {
			case "ping":
				conn.Write([]uint8("pong"))
				if mystate == -1 {
					mystate = 0
				}
			case "l#": //l/token
				mystate = 0
				fmt.Println("set 0")
			case "l!":
				mystate = 1
			case "a#": //a/framelen/nframes/idlen/nids
				mystate = 1
				fmt.Println("set 1")
			case "a!": //a/framelen/nframes/idlen/nids
				mystate = 2
			case "f#":
				mystate = 2
				fmt.Println("set 2")
			case "f!":
				mystate = 3
			case "o#":
				mystate = 5
			default:
				fmt.Println("def:", msg)
				mystate = 0
			}
		default:
			fmt.Println(",")
			switch mystate {
			case -1:
				conn.Write([]uint8("."))
				time.Sleep(500 * time.Millisecond)
				mystate = 0
			case 0:
				dstr := "l/" + token
				//fmt.Println(dstr)
				_, eeee := conn.Write([]uint8(dstr))
				if eeee != nil {
					fmt.Println(eeee)
				} else {
					time.Sleep(500 * time.Millisecond)
				}
				//mystate++
			case 1:
				MyInfo, eee := os.Stat("./data/" + filesend)
				if eee == nil {
					Filelen = uint64(MyInfo.Size())

					data, ee = ioutil.ReadFile("./data/" + filesend)
					if ee == nil {
						FileCRC = 0
						for i = 0; i < uint64(len(data)); i++ {
							FileCRC += uint64(data[i])
						}
						//NFrames = NFRAMES
						plen = int(Filelen/(NFRAMES*NIDS)) + 1
						//plen = int(Filelen / uint64(NIds))
						// a/nframes/idlen/nids
						conn.Write([]uint8("a/" + strconv.Itoa(NFRAMES) + "/" + strconv.Itoa(plen) + "/" + strconv.Itoa(NIDS)))
						time.Sleep(500 * time.Millisecond)
						//fmt.Println("File len", Filelen, "Кол-во фреймов", NFrames, "Кол-во pakets", NIDS, "Длина пакета", plen)
						//mystate++
					} else {
						fmt.Println("quit1")
						ex = 1
						rez = 0
					}
				} else {
					fmt.Println("quit2")
					ex = 1
					rez = 0
				}
			case 2: //f/имя-файла/len/FILEcrc
				asfl := strconv.FormatUint(Filelen, 10)
				ascrc := strconv.FormatUint(FileCRC, 10)
				conn.Write([]uint8("f/" + filesend + "/" + asfl + "/" + ascrc))

				for j = 0; j < len(frames); j++ {
					for k = 0; k < len(frames[j].myIds); k++ {
						frames[j].myIds = make([]nIds, 0)
					}
				}
				ku := uint64(len(data))
				frames = make([]mframes, 0)
				for i = 0; i < ku; {
					for j = 0; j < NFRAMES && i < ku; j++ {
						frameUn.myIds = make([]nIds, 0)
						for k = 0; k < NIDS && i < ku; k++ {

							nid.flag = 0
							nid.offset = i

							if ku > i+uint64(plen) {
								nid.idlen = plen
							} else {
								nid.idlen = int(ku - i)
							}
							i = i + uint64(plen)
							frameUn.myIds = append(frameUn.myIds, nid)
						}
						frameUn.num = j
						frames = append(frames, frameUn)
					}
				}
				time.Sleep(500 * time.Millisecond)
				//mystate++

			case 3: //w/offset/NF/nid/len/data

				f, err := os.Create("./dat.tmp")

				if err == nil {
					for j = len(frames) - 1; j >= 0; j-- {
						for k = 0; k < len(frames[j].myIds) && frames[j].myIds[k].idlen != 0; k++ {
							gstr := "w/" + strconv.FormatUint(frames[j].myIds[k].offset, 10) + "/" + strconv.FormatUint(uint64(j), 10) + "/" + strconv.Itoa(int(k)) + "/" + strconv.Itoa(int(frames[j].myIds[k].idlen))
							fmt.Println(gstr)
							f.Seek(int64(frames[j].myIds[k].offset), 0)
							f.Write(data[int(frames[j].myIds[k].offset) : int(frames[j].myIds[k].offset)+int(frames[j].myIds[k].idlen)])
						}
					}
					f.Close()
				}

				mystate = 10
			case 4:
			case 5:
			}
		}

	}
	fmt.Println("exit handle")
	conn.Close()
	chrez <- rez
}
*/
func mySend(filesend string) bool {
	rez := false

	var chrez chan int
	chrez = make(chan int)
	service := hostName + ":" + portNum
	RemoteAddr, err := net.ResolveUDPAddr("udp", service)
	conn, err := net.DialUDP("udp", nil, RemoteAddr)
	if err == nil {

		go handleUDPConnection(conn, chrez, filesend, RemoteAddr)
		u := <-chrez
		if u == 1 {
			rez = true
		} else {
			rez = false
		}
	} else {
		fmt.Println("Error:", service)
		fmt.Println(err)
	}
	return rez
}

//----------------
type tsktype struct {
	counter  int
	namefile []uint8
}

var myTasks []tsktype
var mut sync.Mutex

func ScanDir() bool {
	var munit tsktype
	var rez bool = true
	var r int = 0
	items, err := ioutil.ReadDir(pathDir)
	if err == nil {
		for i := 0; i < len(items); i++ {
			if items[i].Mode().IsDir() {
				fmt.Println(items[i].Name())
				//ScanDir(pathDir + items[i].Name() + "\\")
			} else {
				// файлы , без символических ссылок
				if items[i].Mode().IsRegular() {

					r = 0
					mut.Lock()
					fmt.Println("&", len(myTasks))
					for j := 0; j < len(myTasks) && r == 0; j++ {
						//fmt.Println(string(myTasks[j].namefile), "<>", items[i].Name())
						if string(myTasks[j].namefile) == items[i].Name() {
							myTasks[j].counter++
							r = 1
						}
					}
					if r == 0 {
						munit.counter = 0
						munit.namefile = []uint8(items[i].Name())
						myTasks = append(myTasks, munit)
					}
					mut.Unlock()
				}
			}

		}
	} else {
		fmt.Println(err)
		rez = false
	}
	return rez
}

func fispetcher() {
	myTasks = make([]tsktype, 0)
	for {
		//		fmt.Println("1.")
		ScanDir()
		//		fmt.Println("2.")

		//		fmt.Println("3.")
		if len(myTasks) > 0 {

			//fmt.Println("#", len(myTasks))
			if myTasks[0].counter > 3 {
				fmt.Println(string(myTasks[0].namefile))
				rezult := mySend(string(myTasks[0].namefile))
				if rezult {
					//удалить или переместить файл
					os.Rename(DATAPATH+string(myTasks[0].namefile), ARCHPATH+string(myTasks[0].namefile))
					os.Remove(DATAPATH + string(myTasks[0].namefile))
					myTasks = myTasks[1:]

				} else {
					time.Sleep(10 * time.Second)
				}
			} else {
				//fmt.Println(">", myTasks[0].counter)
			}
			time.Sleep(3 * time.Second)

		}

		time.Sleep(time.Second)
	}
}

func main() {
	go fispetcher()

	//LocalAddr := nil
	// see https://golang.org/pkg/net/#DialUDP

	// note : you can use net.ResolveUDPAddr for LocalAddr as well
	//        for this tutorial simplicity sake, we will just use nil
	for {
		time.Sleep(5 * time.Second)
	}
}
