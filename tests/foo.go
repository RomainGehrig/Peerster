package main

import "fmt"
import "time"

func runDispatcher() (chan<- int, chan (chan<- int), chan (chan<- int)) {
	input := make(chan int, 100)
	registerChan := make(chan (chan<- int))
	unregisterChan := make(chan (chan<- int))

	go func() {
		waiters := make(map[chan<- int]bool)

		for {
			select {
			case newChan := <-registerChan:
				waiters[newChan] = true
				fmt.Println("Channel registers:", newChan)
			case toClose := <-unregisterChan:
				// Important part: the dispatcher closes the receiver's channel
				fmt.Println("Channel wants to unregister", toClose)
				delete(waiters, toClose)
				close(toClose)
			case val := <-input:
				fmt.Println("Sending", val, "to all channels")
				for c, _ := range waiters {
					fmt.Println("Sending", val, "to", c)
					// Important to be non blocking ?
					// => Can be blocking if we add buffering to input channels
					select {
					case c <- val:
						fmt.Println("Sent", val, "to", c)
						// default:
						// 	fmt.Println("Did not send", val, "to", c)
					}
				}
			}
		}

	}()

	return input, registerChan, unregisterChan
}

func main() {

	input, registerChan, unregisterChan := runDispatcher()

	channels := [6]int{1, 2, 3, 4, 5, 6}

	for _, v := range channels {
		go func(name int, registerChan, unregisterChan chan (chan<- int)) {
			self_chan := make(chan int, 10)

			// Close is done by supervisor
			// defer close(self_chan)

			// Inform supervisor that the goroutine exists
			registerChan <- self_chan

			for {
				select {
				case val, ok := <-self_chan:
					// !ok means the channel was closed => we stop everything
					if !ok {
						return
					}

					fmt.Println("Channel", name, "has received value", val)
					if val == name {
						fmt.Println("Go routine", name, "will unregister")
						unregisterChan <- self_chan
					}
				}
			}

		}(v, registerChan, unregisterChan)
	}
	time.Sleep(1 * time.Second)

	for v, _ := range channels {
		fmt.Println("Want to send", v, "to all channels")
		input <- v
	}
	time.Sleep(1 * time.Second)
}
