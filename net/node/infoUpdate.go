package node

import (
	"dad-go/common"
	. "dad-go/net/message"
	. "dad-go/net/protocol"
	"time"
)

func keepAlive(from *Noder, dst *Noder) {
	// Need move to node function or keep here?
}

func (node node) GetBlkHdrs() {
	for _, n := range node.local.neighb.List {
		h1 := n.GetHeight()
		h2 := node.local.GetHeight()
		if (node.GetState() == ESTABLISH) && (h1 > h2) {
			//buf, _ := newMsg("version")
			buf, _ := NewMsg("getheaders", node.local)
			//buf, _ := newMsg("getaddr")
			go node.Tx(buf)
		}
	}
}

func (node node) ReqNeighborList() {
	buf, _ := NewMsg("getaddr", node.local)
	go node.Tx(buf)
}

// Fixme the Nodes should be a parameter
func (node node) updateNodeInfo() {
	ticker := time.NewTicker(time.Second * PERIODUPDATETIME)
	quit := make(chan struct{})

	for {
		select {
		case <-ticker.C:
			common.Trace()
			node.GetBlkHdrs()
		case <-quit:
			ticker.Stop()
			return
		}
	}
	// TODO when to close the timer
	//close(quit)
}