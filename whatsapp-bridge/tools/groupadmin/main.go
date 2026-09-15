// groupadmin — one-off helper: uses a SEPARATE whatsmeow store (the backed-up
// session of Nicky's own account) to check group membership and add a member.
//   go run ./tools/groupadmin -store <whatsapp.db> -check
//   go run ./tools/groupadmin -store <whatsapp.db> -add 4917xxxx -groups jid1,jid2
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
)

func main() {
	store := flag.String("store", "", "path to whatsapp.db of the session to use")
	check := flag.Bool("check", false, "only report login + groups")
	add := flag.String("add", "", "phone number (no +) to add")
	groups := flag.String("groups", "", "comma-separated group JIDs")
	flag.Parse()
	if *store == "" { fmt.Println("need -store"); os.Exit(2) }

	ctx := context.Background()
	container, err := sqlstore.New(ctx, "sqlite3", "file:"+*store+"?_foreign_keys=on", waLog.Noop)
	if err != nil { fmt.Println("store:", err); os.Exit(1) }
	dev, err := container.GetFirstDevice(ctx)
	if err != nil || dev == nil || dev.ID == nil { fmt.Println("NO-SESSION in store"); os.Exit(1) }
	client := whatsmeow.NewClient(dev, waLog.Noop)
	if err := client.Connect(); err != nil { fmt.Println("connect:", err); os.Exit(1) }
	defer client.Disconnect()
	deadline := time.Now().Add(15 * time.Second)
	for !client.IsLoggedIn() && time.Now().Before(deadline) { time.Sleep(300 * time.Millisecond) }
	if !client.IsLoggedIn() { fmt.Println("SESSION-DEAD (not logged in after 15s)"); os.Exit(1) }
	fmt.Println("LOGGED-IN as", dev.ID.String())

	if *check {
		gs, err := client.GetJoinedGroups(ctx)
		if err != nil { fmt.Println("groups:", err); os.Exit(1) }
		fmt.Printf("JOINED-GROUPS %d\n", len(gs))
		for _, g := range gs { fmt.Printf("  %s\t%s\n", g.JID.String(), g.Name) }
		return
	}
	if *add != "" && *groups != "" {
		target := types.NewJID(*add, types.DefaultUserServer)
		for _, j := range strings.Split(*groups, ",") {
			gj, err := types.ParseJID(strings.TrimSpace(j))
			if err != nil { fmt.Println("bad jid", j); continue }
			res, err := client.UpdateGroupParticipants(ctx, gj, []types.JID{target}, whatsmeow.ParticipantChangeAdd)
			if err != nil { fmt.Printf("FAIL %s: %v\n", gj, err); continue }
			// r.Error is a numeric status: 0 = added, 409 = already a member, 403 = not admin
			for _, r := range res { fmt.Printf("%s %s -> %d\n", gj, r.JID, r.Error) }
			time.Sleep(1200 * time.Millisecond) // don't look like a spam bot
		}
	}
}
