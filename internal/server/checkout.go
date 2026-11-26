package server

import "net/http"

func checkoutHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(checkoutHTML))
}

const checkoutHTML = `<!doctype html>
<html lang="id"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Payment Sandbox</title><style>body{font:16px system-ui;max-width:600px;margin:40px auto;padding:20px}input,button{display:block;padding:10px;margin:8px 0;width:100%;box-sizing:border-box}pre{white-space:pre-wrap}button{cursor:pointer}</style>
<h1>Payment Sandbox</h1><p>Gunakan data dan dana simulasi saja.</p>
<form id="form"><label>Nama<input name="name" required></label><label>Email<input name="email" type="email" required></label><label>Nominal IDR<input name="amount" type="number" min="1" max="100000000" required value="125000"></label><button>Buat order dan pembayaran</button></form>
<p><a id="pay" hidden target="_blank" rel="noopener noreferrer">Buka Midtrans Sandbox</a></p>
<label>Nomor order<input id="number"></label><button id="check">Cek status</button><pre id="output" role="status"></pre>
<script>
const form=document.querySelector('#form'), number=document.querySelector('#number'), output=document.querySelector('#output'), pay=document.querySelector('#pay');
let key=crypto.randomUUID(), payload='';
async function call(path, options){const res=await fetch('/api/v1'+path,options);const data=await res.json();if(!res.ok)throw Error(data.error?.message||res.status);return data}
form.onsubmit=async e=>{e.preventDefault();const button=form.querySelector('button');button.disabled=true;pay.hidden=true;try{const f=new FormData(form), body=JSON.stringify({customer:{name:f.get('name'),email:f.get('email')},amount:Number(f.get('amount')),currency:'IDR'});if(payload&&payload!==body){key=crypto.randomUUID();number.value=''}payload=body;const order=await call('/orders',{method:'POST',headers:{'Content-Type':'application/json','Idempotency-Key':key},body});number.value=order.orderNumber;const p=await call('/orders/'+encodeURIComponent(number.value)+'/payments',{method:'POST'});const u=new URL(p.redirectUrl);if(u.protocol!=='https:'||u.hostname!=='app.sandbox.midtrans.com')throw Error('Invalid checkout URL');pay.href=u.href;pay.hidden=false;output.textContent=labels[p.status]||p.status;}catch(e){output.textContent=e.message}finally{button.disabled=false}};
const labels={pending:'Menunggu pembayaran',authorize:'Pembayaran diotorisasi',capture:'Pembayaran berhasil',settlement:'Pembayaran berhasil',cancel:'Pembayaran dibatalkan',expire:'Pembayaran kedaluwarsa',deny:'Pembayaran ditolak',failure:'Pembayaran gagal',refund:'Dana dikembalikan',partial_refund:'Dana dikembalikan sebagian',chargeback:'Pembayaran disengketakan'};
document.querySelector('#check').onclick=async()=>{const button=document.querySelector('#check');button.disabled=true;pay.hidden=true;output.textContent='Memeriksa status ke Midtrans...';try{const p=await call('/orders/'+encodeURIComponent(number.value.trim())+'/payments/refresh',{method:'POST'});output.textContent=(labels[p.status]||p.status)+'\nNomor order: '+p.orderNumber;if(['cancel','expire','deny','failure'].includes(p.status)){output.textContent+='\nSilakan buat order baru jika ingin melanjutkan pembayaran.';key=crypto.randomUUID();payload='';}}catch(e){output.textContent='Status terbaru belum dapat dikonfirmasi. '+e.message}finally{button.disabled=false}};
</script></html>`
