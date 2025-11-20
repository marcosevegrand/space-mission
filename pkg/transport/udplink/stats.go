package udplink

import (
	"fmt"
	"runtime"
	"time"
)

// statsLoop é um loop de monitorização que imprime periodicamente as estatísticas
// do estado interno do Peer para o standard output.
// É útil para depuração e para detetar potenciais leaks de memória.
func (p *Peer[T]) statsLoop() {
	// Garante que o WaitGroup é decrementado quando a goroutine termina.
	defer p.wg.Done()

	// Cria um ticker que dispara a cada segundo.
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop() // Liberta os recursos do ticker quando a função termina.

	for {
		select {
		// Se o canal de paragem for fechado, termina a goroutine.
		case <-p.stopChan:
			// Imprime uma linha em branco para limpar a linha de estatísticas final.
			fmt.Println()
			return

		// Quando o ticker dispara.
		case <-ticker.C:
			// Bloqueia o mutex para ler os tamanhos dos mapas de forma segura.
			p.pktMu.Lock()
			pendingSent := len(p.sentPkts)
			incompleteRecv := len(p.recvPkts)
			p.pktMu.Unlock()

			// Obtém o número atual de goroutines em execução.
			// Um aumento contínuo e ilimitado aqui é um sinal claro de "goroutine leak".
			numGoroutines := runtime.NumGoroutine()

			// Obtém estatísticas de memória do runtime do Go.
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			// HeapAlloc é a memória que o heap está a utilizar atualmente.
			// Um aumento contínuo e ilimitado aqui indica um leak de memória no heap.
			heapAllocMiB := memStats.HeapAlloc / 1024 / 1024

			// Imprime as estatísticas formatadas numa única linha.
			// O `\r` no início faz com que o cursor volte ao início da linha,
			// permitindo que cada print substitua o anterior, criando um display dinâmico.
			// Os especificadores de formatação como `%-5d` alinham os números para que a linha não "salte".
			fmt.Printf(
				"[STATS] Pending Sent: %-5d | Incomplete Recv: %-5d | Goroutines: %-4d | Heap: %-4d MiB\n",
				pendingSent,
				incompleteRecv,
				numGoroutines,
				heapAllocMiB,
			)
		}
	}
}
