// Samler mikrofonlyd (Float32, mono) og sender den til hovedtråden i bidder.
// Lyden forlader aldrig computeren – den sendes kun til OpenLST på localhost.
class RecorderProcessor extends AudioWorkletProcessor {
  constructor() {
    super();
    this.buf = new Float32Array(4096);
    this.len = 0;
    this.port.onmessage = (e) => {
      if (e.data === 'flush') {
        this.flush();
        this.port.postMessage('flushed');
      }
    };
  }

  flush() {
    if (this.len > 0) {
      this.port.postMessage(this.buf.slice(0, this.len));
      this.len = 0;
    }
  }

  process(inputs) {
    const ch = inputs[0] && inputs[0][0];
    if (ch) {
      for (let i = 0; i < ch.length; i++) {
        this.buf[this.len++] = ch[i];
        if (this.len === this.buf.length) this.flush();
      }
    }
    return true;
  }
}

registerProcessor('recorder', RecorderProcessor);
