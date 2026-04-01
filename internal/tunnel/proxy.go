package tunnel

import (
	"context"
	"io"
	"net"
	"sync"
)

// bufferPool reutiliza los buffers para los túneles TCP y reduce la presión
// sobre el Garbage Collector. Tamaño estándar de 32KB.
var bufferPool = sync.Pool{
	New: func() any {
		b := make([]byte, 32*1024)
		return &b
	},
}

// Proxy copia los datos de forma bidireccional entre dos conexiones de red.
// Utiliza un context de forma tal que si se cancela la orden, rompe las lecturas.
func Proxy(ctx context.Context, c1, c2 net.Conn) error {
	return ProxyWithTraffic(ctx, c1, c2, nil, nil)
}

// ProxyWithTraffic copia los datos en ambos sentidos, e informa el tráfico copiado
// por dirección con callbacks opcionales.
//
// - onC1toC2 se invoca con bytes copiados desde c1 hacia c2
// - onC2toC1 se invoca con bytes copiados desde c2 hacia c1
func ProxyWithTraffic(ctx context.Context, c1, c2 net.Conn, onC1toC2 func(n int64), onC2toC1 func(n int64)) error {
	defer func() {
		c1.Close()
		c2.Close()
	}()

	var wg sync.WaitGroup
	errc := make(chan error, 2)

	// Inicia la copia en ambos sentidos concurrentemente.
	wg.Add(2)
	go copyConn(c1, c2, &wg, errc, onC2toC1)
	go copyConn(c2, c1, &wg, errc, onC1toC2)

	// Espera la conclusión natural o la cancelación del contexto.
	go func() {
		wg.Wait()
		close(errc)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errc:
		return err
	}
}

// copyConn toma un pool de bytes y maneja la escritura un solo lado del túnel
func copyConn(dst, src net.Conn, wg *sync.WaitGroup, errc chan<- error, onCopied func(n int64)) {
	defer wg.Done()

	bufPtr := bufferPool.Get().(*[]byte)
	defer bufferPool.Put(bufPtr)

	b := *bufPtr
	for {
		nr, er := src.Read(b)
		if nr > 0 {
			written := 0
			for written < nr {
				nw, ew := dst.Write(b[written:nr])
				if nw > 0 {
					written += nw
					if onCopied != nil {
						onCopied(int64(nw))
					}
				}
				if ew != nil {
					if ew != io.EOF {
						errc <- ew
					}
					return
				}
				if nw == 0 {
					errc <- io.ErrUnexpectedEOF
					return
				}
			}
		}
		if er != nil {
			if er != io.EOF {
				errc <- er
			}
			return
		}
	}
}
