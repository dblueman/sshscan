package main

import (
   "context"
   "flag"
   "fmt"
   "net"
   "net/netip"
   "os"
   "strings"
   "sync"
   "time"

   "golang.org/x/crypto/ssh"
   "golang.org/x/term"
)

var (
   wg    sync.WaitGroup
   hosts sync.Map
)

func main() {
   flag.Usage = func() {
      fmt.Fprint(os.Stderr, "usage: <subnet> [command] ...\n" +
         "   eg sshscan 10.1.2.0/24\n")
      flag.PrintDefaults()
   }

   flag.Parse()

   if flag.NArg() < 1 {
      flag.Usage()
      os.Exit(2)
   }

   prefix, err := netip.ParsePrefix(flag.Arg(0))
   if err != nil {
      flag.Usage()
      os.Exit(2)
   }

   cmd := strings.Join(flag.Args()[1:], " ")

   err = scan(prefix, cmd)
   if err != nil {
      fmt.Println(os.Stderr, err.Error())
      os.Exit(1)
   }

   fmt.Println("\naccessible hosts:")
   hosts.Range(func(host, out any) bool {
      fmt.Printf("%s: %s", host.(string), out.(string))
      return true
   })
}

func scan(prefix netip.Prefix, cmd string) error {
   addr := prefix.Addr()

   fmt.Print("username: ")
   var user string
   if _, err := fmt.Scanf("%s", &user); err != nil {
      return fmt.Errorf("scan: %w", err)
   }

   fmt.Print("password: ")

   pass, err := term.ReadPassword(int(os.Stdin.Fd()))
   if err != nil {
      return fmt.Errorf("scan: %w", err)
   }
   fmt.Print("\nscanning...")

   for {
      if !prefix.Contains(addr) {
         break
      }

      wg.Add(1)
      go try(addr.String(), user, string(pass), cmd)
      time.Sleep(30 * time.Millisecond)

      addr = addr.Next()
   }

   wg.Wait()

   return nil
}

func try(ip, user, pass, cmd string) {
   defer wg.Done()

   addr := ip + ":22"
   sshConfig := ssh.ClientConfig{
      User:            user,
      Auth:            []ssh.AuthMethod{ssh.Password(pass)},
      HostKeyCallback: ssh.InsecureIgnoreHostKey(),
   }

   d := net.Dialer{}
   ctx, cancel := context.WithTimeout(context.Background(), 45 * time.Second)
   defer cancel()

   conn, err := d.DialContext(ctx, "tcp", addr)
   if err != nil {
      return
   }
   defer conn.Close()

   c, chans, reqs, err := ssh.NewClientConn(conn, addr, &sshConfig)
   if err != nil {
      return
   }
   defer c.Close()

   client := ssh.NewClient(c, chans, reqs)
   session, err := client.NewSession()
   if err != nil {
      return
   }
   defer session.Close()

   out, _ := session.CombinedOutput(cmd)
   hosts.Store(ip, string(out))
}
