package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/user/bifrost/internal/crypto"
	"golang.org/x/term"
)

func newEncryptCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "encryptsecret",
		Short: "Genera un hash encriptado para almacenar de manera segura en tu config / .conf",
		Run: func(cmd *cobra.Command, args []string) {
			secretBytes, err := readPasswordWithMask()
			if err != nil {
				fmt.Printf("\nFallo interceptando la entrada: %v\n", err)
				os.Exit(1)
			}
			
			secret := string(secretBytes)

			// Limpiamos los saltos de carro del enter (\n o \r\n)
			secret = strings.TrimSpace(secret)
			if secret == "" {
				fmt.Println("Error: No puedes encriptar un string vacío.")
				os.Exit(1)
			}

			// Pasamos el string a la capa de encriptación simétrica AES-GCM
			hash, err := crypto.Encrypt(secret)
			if err != nil {
				fmt.Printf("Fallo al cifrar AES-GCM: %v\n", err)
				os.Exit(1)
			}

			fmt.Println("\n=== Hash AES Generado ===")
			fmt.Printf("Token Seguro: %s\n", hash)
			fmt.Println("\nPara usarlo colócalo en tu config/server.conf de la siguiente forma:")
			fmt.Printf("   \"secret\": \"ENC:%s\"\n", hash)
		},
	}
}

// readPasswordWithMask pone la terminal en modo Raw para interceptar los tipeos 
// y ofuscarlos uno a uno imprimiendo '*', simulando el comportamiento de sudo 
// o un prompt clásico, pero permitiendo ver de manera ofuscada qué se introdujo.
func readPasswordWithMask() ([]byte, error) {
	fmt.Print("Ingresa el texto a encriptar: ")

	// Tomamos el control de la terminal
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		// Fallback: Si no estamos en una terminal TTY, usamos term.ReadPassword estándar
		// (por ejemplo si la entrada es inyectada en un pipeline echo "password" | ./bifrost).
		return term.ReadPassword(fd)
	}
	defer term.Restore(fd, oldState)

	var password []byte
	buf := make([]byte, 1)

	for {
		_, err := os.Stdin.Read(buf)
		if err != nil {
			return nil, err
		}

		c := buf[0]

		// Finaliza con Enter (\r)
		if c == '\n' || c == '\r' {
			fmt.Print("\r\n") // Imprime nueva línea real
			break
		}

		// Interrumpe con Ctrl+C
		if c == 3 {
			fmt.Print("\r\n")
			return nil, fmt.Errorf("operación abortada")
		}

		// Manejo correcto de Backspace/Retroceso
		if c == 127 || c == '\b' {
			if len(password) > 0 {
				password = password[:len(password)-1]
				// ANSI escape: mueve cursor atrás, borra y vuelve atrás
				fmt.Print("\b \b")
			}
			continue
		}

		// Almacenamos y ofuscamos con '*'
		password = append(password, c)
		fmt.Print("*")
	}

	return password, nil
}
