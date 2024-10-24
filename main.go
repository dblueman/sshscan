package main

import (
   "flag"
   "fmt"
   "net/netip"
   "os"
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
      fmt.Fprint(os.Stderr, "usage: [options] <subnet>\n" +
         "   eg sshscan 10.1.2.0/24\n")
      flag.PrintDefaults()
   }

   flag.Parse()

   if flag.NArg() != 1 {
      flag.Usage()
      os.Exit(2)
   }

   prefix, err := netip.ParsePrefix(flag.Arg(0))
   if err != nil {
      flag.Usage()
      os.Exit(2)
   }

   err = scan(prefix)
   if err != nil {
      fmt.Println(os.Stderr, err.Error())
      os.Exit(1)
   }

   fmt.Println("\naccessible hosts:")
   hosts.Range(func(host, _ any) bool {
      fmt.Println(host.(string))
      return true
   })
}

func scan(prefix netip.Prefix) error {
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
      go try(addr.String(), user, string(pass))
      time.Sleep(30 * time.Millisecond)

      addr = addr.Next()
   }

   wg.Wait()

   return nil
}

func try(ip, user, pass string) {
   defer wg.Done()

   sshConfig := ssh.ClientConfig{
      User:            user,
      Auth:            []ssh.AuthMethod{ssh.Password(pass)},
      HostKeyCallback: ssh.InsecureIgnoreHostKey(),
      Timeout:         3 * time.Second,
   }

   _, err := ssh.Dial("tcp", ip + ":22", &sshConfig)
   if err == nil {
      hosts.Store(ip, "")
   }
}
