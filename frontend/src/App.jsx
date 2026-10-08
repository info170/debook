import { useEffect, useRef, useState } from 'react'
import { Activity, ArrowDown, ArrowUpRight, Ban, Check, ChevronDown, CircleHelp, Clock3, Code2, ExternalLink, Gauge, Globe2, Layers3, Play, RotateCcw, Settings2, ShieldCheck, Square, Zap } from 'lucide-react'

const isRussian = window.location.pathname === '/ru' || window.location.pathname.startsWith('/ru/')
const text = (ru, en) => isRussian ? ru : en
const english = {
  'Запуск готов к работе':'Run is ready','Настройте сценарий и нажмите «Запустить тест»':'Configure the scenario and click “Run test”',
  'Инструменты':'Tools','Локальное окружение':'Local environment','Нагрузочное тестирование':'Load testing','Проверка сценария бронирования в реальном времени':'Real-time booking scenario validation','Конфигурация сценария':'Scenario configuration','Задайте параметры тестового прогона':'Set the parameters for your test run','Интенсивность':'Rate','СЕАНСОВ В СЕКУНДУ':'SESSIONS PER SECOND','запросов / сек':'requests / sec','Щадящий прогон':'Light run','Средняя нагрузка':'Medium load','Интенсивный прогон':'Intensive run','Высокая нагрузка':'High load','Длительность теста':'Test duration','МИНУТЫ : СЕКУНДЫ':'MINUTES : SECONDS','Всего до':'Up to','сеансов':'sessions','СЦЕНАРИЙ':'SCENARIO','АВТОМАТИЧЕСКИ':'AUTOMATED','Получить слоты':'Fetch slots','Забронировать слот':'Book a slot','Отменить бронь':'Cancel booking','Параметры API':'API parameters','Изменить':'Edit','Скрыть':'Hide','Запустить тест':'Run test','Остановить тест':'Stop test','Безопасно для локальной среды':'Safe for local environment','Результаты прогона':'Run results','Текущий тестовый запуск':'Current test run','В ПРОЦЕССЕ':'RUNNING','ОЖИДАНИЕ':'IDLE','ЗАПУЩЕНО СЕАНСОВ':'SESSIONS STARTED','СЛОТОВ ПОЛУЧЕНО':'SLOTS FETCHED','СОЗДАНО БРОНЕЙ':'BOOKINGS CREATED','ОТМЕНЕНО БРОНЕЙ':'BOOKINGS CANCELLED','Прогресс теста':'Test progress','Успешно:':'Successful:','Ошибки:':'Errors:','Средняя скорость:':'Average rate:','Активность запросов':'Request activity','Хронология операций теста':'Test operation timeline','Очистить':'Clear','событий':'events','События обновляются в реальном времени':'Events update in real time','Тестируйте ответственно':'Test responsibly','OpenAPI документация':'OpenAPI documentation','Нагрузочный тест запущен':'Load test started','Тест завершён по времени':'Test completed on schedule','Тест остановлен':'Test stopped','Подключение к API':'API connection','Получено свободных слотов':'Available slots fetched','Бронь создана':'Booking created','Бронирование отменено':'Booking cancelled','Ошибка отмены':'Cancellation error','слоты не найдены':'no slots found','Не удалось подключиться к API':'Could not connect to API'
}

const initialEvents = [
  { time: '14:32:08.421', method: 'GET', title: 'Запуск готов к работе', detail: 'Настройте сценарий и нажмите «Запустить тест»', status: 'ready' },
  { time: '14:32:08.419', method: 'SYS', title: 'Подключение к API', detail: 'http://127.0.0.1:8080', status: 'ok' },
]

function App() {
  const [rate, setRate] = useState(10)
  const [rateInput, setRateInput] = useState('10')
  const [duration, setDuration] = useState(60)
  const [apiUrl, setApiUrl] = useState('http://127.0.0.1:8080')
  const [region, setRegion] = useState('eu-1')
  const [resource, setResource] = useState('room-1')
  const [customerRef, setCustomerRef] = useState('load-test-client')
  const [wallet, setWallet] = useState('0x8bbA1Fa0F82E5ec07B06950578abA23B7f52a285')
  const [running, setRunning] = useState(false)
  const [elapsed, setElapsed] = useState(0)
  const [counts, setCounts] = useState({ sessions: 0, slots: 0, booked: 0, cancelled: 0, errors: 0 })
  const [events, setEvents] = useState(initialEvents)
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [notice, setNotice] = useState('')
  useEffect(() => {
    if (isRussian) return
    const translate = () => {
      const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT)
      const nodes = []
      while (walker.nextNode()) nodes.push(walker.currentNode)
      nodes.forEach(node => { const value = node.nodeValue.trim(); if (english[value]) node.nodeValue = node.nodeValue.replace(value, english[value]) })
    }
    translate()
    const observer = new MutationObserver(translate)
    observer.observe(document.getElementById('root'), { childList: true, subtree: true })
    return () => observer.disconnect()
  }, [])
  const timer = useRef(null)
  const active = useRef(false)
  const issued = useRef(0)
  const sessionNum = useRef(0)
  const roundRobin = useRef(0)
  const slots = useRef([])
  const bookings = useRef([])
  const eventsRef = useRef(null)
  const startTime = useRef(0)
  const successfulBookings = useRef(0)
  const inFlight = useRef(0)
  const maxInFlight = useRef(20)
  const controllers = useRef(new Set())

  useEffect(() => () => { active.current = false; clearInterval(timer.current) }, [])
  useEffect(() => { if (eventsRef.current) eventsRef.current.scrollTop = 0 }, [events])

  const addEvent = (method, title, detail, status = 'ok') => {
    if (!isRussian) {
      Object.entries(english).forEach(([ru, en]) => { title = title.replaceAll(ru, en); detail = detail.replaceAll(ru, en) })
    }
    const now = new Date()
    const stamp = now.toLocaleTimeString('ru-RU', { hour12: false }) + '.' + String(now.getMilliseconds()).padStart(3, '0')
    setEvents(prev => [{ time: stamp, method, title, detail, status }, ...prev].slice(0, 120))
  }
  const bump = key => setCounts(prev => ({ ...prev, [key]: prev[key] + 1 }))
  const updateRate = raw => {
    setRateInput(raw)
    if (raw === '') return
    const next = Math.max(1, Math.min(1000, Number(raw) || 1))
    setRate(next)
  }
  const request = async (path, options = {}, controller) => {
    const response = await fetch(`${apiUrl.replace(/\/$/, '')}${path}`, { ...options, signal: controller?.signal, headers: { ...(options.body ? { 'Content-Type': 'application/json' } : {}), ...(options.headers || {}) } })
    const body = await response.json().catch(() => ({}))
    if (!response.ok) throw new Error(body.error || `HTTP ${response.status}`)
    return body
  }
  const cancelOne = async booking => {
    try {
      const body = await request(`/api/v1/bookings/${booking.id}/cancel`, { method: 'POST' })
      bump('cancelled')
      addEvent('POST', 'Бронирование отменено', `${booking.id.slice(0, 8)} · ${body.status || 'CANCELLED'}`, 'ok')
    } catch (error) {
      bump('errors')
      addEvent('POST', 'Ошибка отмены', error.message, 'error')
    }
  }
  const runSession = async () => {
    if (!active.current) return
    const number = ++sessionNum.current
    inFlight.current += 1
    const controller = new AbortController()
    controllers.current.add(controller)
    bump('sessions')
    try {
      let available = slots.current.filter(slot => slot.status === 'AVAILABLE')
      if (!available.length) {
        const result = await request(`/api/v1/slots?region_id=${encodeURIComponent(region)}&resource_id=${encodeURIComponent(resource)}&status=AVAILABLE`, {}, controller)
        if (!active.current) return
        bump('slots')
        available = result.filter(slot => slot.status === 'AVAILABLE')
        slots.current = [...slots.current.filter(slot => slot.status !== 'AVAILABLE'), ...available]
        if (!available.length) {
          addEvent('GET', `Сессия #${number}: слоты не найдены`, 'Нет свободных слотов для выбранного региона и ресурса', 'error')
          bump('errors')
          return
        }
        addEvent('GET', `Получено ${available.length} свободных слотов`, `${region} · ${resource}`, 'ok')
      }
      const slot = available[roundRobin.current % available.length]
      if (!active.current) return
      roundRobin.current += 1
      slots.current = slots.current.map(item => item.id === slot.id ? { ...item, status: 'RESERVED' } : item)
      const booking = await request('/api/v1/bookings', {
        method: 'POST',
        headers: { 'Idempotency-Key': crypto.randomUUID() },
        body: JSON.stringify({ slot_id: slot.id, customer_ref: `${customerRef}-${number}`, wallet_address: wallet }),
      }, controller)
      // A request already sent cannot be recalled by the browser. If it completed
      // after Stop, leave the booking as-is and do not start follow-up requests.
      if (!active.current) {
        addEvent('POST', `Бронь создана · ${booking.status}`, `${booking.id.slice(0, 8)} · получена после остановки`, 'ready')
        return
      }
      successfulBookings.current += 1
      bump('booked')
      bookings.current.push(booking)
      addEvent('POST', `Бронь создана · ${booking.status}`, `${booking.id.slice(0, 8)} · слот ${slot.id.slice(0, 8)}`, 'ok')
      if (successfulBookings.current % 10 === 0) await cancelOne(booking)
    } catch (error) {
      if (error.name === 'AbortError') return
      bump('errors')
      addEvent('POST', `Сессия #${number}: ошибка`, error.message, 'error')
    } finally {
      inFlight.current = Math.max(0, inFlight.current - 1)
      controllers.current.delete(controller)
    }
  }
  const countsRef = useRef(counts)
  countsRef.current = counts
  const start = async () => {
    if (running) return
    if (!apiUrl.trim() || !region.trim() || !resource.trim() || !customerRef.trim() || !wallet.trim()) {
      setNotice('Заполните адрес API, регион, ресурс, customer ref и EVM-кошелёк.')
      return
    }
    setNotice('')
    try {
      const response = await fetch(`${apiUrl.replace(/\/$/, '')}/healthz`)
      if (!response.ok) throw new Error(`HTTP ${response.status}`)
    } catch (error) {
      setNotice(`API недоступен: ${error.message}`)
      addEvent('SYS', 'Не удалось подключиться к API', error.message, 'error')
      return
    }
    setCounts({ sessions: 0, slots: 0, booked: 0, cancelled: 0, errors: 0 })
    countsRef.current = { sessions: 0, slots: 0, booked: 0, cancelled: 0, errors: 0 }
    setElapsed(0)
    sessionNum.current = 0
    inFlight.current = 0
    issued.current = 0
    successfulBookings.current = 0
    roundRobin.current = 0
    slots.current = []
    bookings.current = []
    active.current = true
    setRunning(true)
    addEvent('SYS', 'Нагрузочный тест запущен', text(`${rate} сессий/с · ${duration} сек.`, `${rate} sessions/sec · ${duration} sec.`), 'ok')
    timer.current = setInterval(() => {
      if (!active.current) return
      const now = Date.now()
      const seconds = Math.floor((now - startTime.current) / 1000)
      setElapsed(seconds)
      const target = Math.min(rate * duration, Math.floor((now - startTime.current) / 1000 * rate))
      let launchCount = Math.min(target - issued.current, maxInFlight.current - inFlight.current)
      while (launchCount > 0) {
        issued.current++
        launchCount--
        void runSession()
      }
      if (seconds >= duration) stop('Тест завершён по времени')
    }, 30)
    startTime.current = Date.now()
    // Fire the first session immediately instead of waiting for the next rate tick.
    for (let i = 0; i < Math.min(rate, maxInFlight.current); i++) {
      if (issued.current >= rate * duration) break
      issued.current++
      void runSession()
    }
  }
  const stop = (message = 'Тест остановлен') => {
    if (!active.current) return
    active.current = false
    clearInterval(timer.current)
    controllers.current.forEach(controller => controller.abort())
    controllers.current.clear()
    setRunning(false)
    addEvent('SYS', message, text(`${issued.current} сессий отправлено · ${inFlight.current} запросов/сессий ещё завершаются`, `${issued.current} sessions sent · ${inFlight.current} requests/sessions still completing`), 'ready')
  }
  const progress = Math.min(100, duration ? elapsed / duration * 100 : 0)
  const rateLabel = rate >= 1000 ? 'Высокая нагрузка' : rate >= 500 ? 'Интенсивный прогон' : rate >= 100 ? 'Средняя нагрузка' : 'Щадящий прогон'

  return <div className="app-shell">
    <aside className="rail">
      <div className="brand-mark"><Layers3 size={20} strokeWidth={2.4} /></div>
      <div className="rail-divider" />
      <button className="rail-icon active" aria-label="Нагрузочное тестирование"><Gauge size={19} /></button>
      <button className="rail-icon" aria-label="Запросы"><Code2 size={19} /></button>
      <button className="rail-icon" aria-label="Активность"><Activity size={19} /></button>
      <div className="rail-bottom"><button className="rail-icon" aria-label="Справка"><CircleHelp size={19} /></button><div className="avatar">D</div></div>
    </aside>
    <main className="main-area">
      <header className="topbar"><div className="breadcrumbs"><span>Инструменты</span><span className="slash">/</span><strong>Load Lab</strong></div><div className="top-right"><div className="environment"><span className="pulse-dot" />Локальное окружение</div><div className="top-avatar">DL</div></div></header>
      <div className="content-wrap">
        <section className="intro"><div><div className="eyebrow"><Zap size={13} fill="currentColor" /> PERFORMANCE TOOLKIT</div><h1>Нагрузочное тестирование</h1><p className="subtitle">Проверка сценария бронирования в реальном времени</p></div><div className="api-pill"><Globe2 size={15} /><span>API endpoint</span><code>{apiUrl.replace(/^https?:\/\//, '')}</code><span className="online-dot" /></div></section>
        <section className="dashboard-grid">
          <div className="left-column">
            <section className="panel config-panel">
              <div className="panel-heading"><div className="heading-icon violet"><Settings2 size={17} /></div><div><h2>Конфигурация сценария</h2><p>Задайте параметры тестового прогона</p></div><span className="step-tag">STEP 01</span></div>
              <div className="field-label-row"><label htmlFor="rate">Интенсивность</label><span className="field-meta">СЕАНСОВ В СЕКУНДУ</span></div>
              <div className="rate-control"><button className="square-button" disabled={running} onClick={() => { const n = Math.max(1, rate - (rate > 100 ? 50 : 1)); setRate(n); setRateInput(String(n)) }}>−</button><div className="rate-input-wrap"><input id="rate" inputMode="numeric" value={rateInput} disabled={running} onChange={e => updateRate(e.target.value)} onBlur={() => { if (!rateInput) setRateInput(String(rate)) }} /><span>запросов / сек</span></div><button className="square-button" disabled={running} onClick={() => { const n = Math.min(1000, rate + (rate >= 100 ? 50 : 1)); setRate(n); setRateInput(String(n)) }}>+</button></div>
              <div className="slider-wrap"><input aria-label={text('Интенсивность сессий в секунду', 'Sessions per second')} type="range" min="1" max="1000" value={rate} disabled={running} style={{ '--range-progress': `${rate / 10}%` }} onChange={e => { setRate(Number(e.target.value)); setRateInput(e.target.value) }} /><div className="range-labels"><span>1</span><span>250</span><span>500</span><span>750</span><span>{text('1000 макс.', '1000 max')}</span></div></div>
              <div className="rate-caption"><span className="caption-dot" />{rateLabel}<span className="caption-note">{text(`до ${rate * 2} HTTP-запросов / сек`, `up to ${rate * 2} HTTP requests / sec`)}</span></div>
              <div className="field-label-row duration-label"><label>Длительность теста</label><span className="field-meta">МИНУТЫ : СЕКУНДЫ</span></div>
              <div className="duration-row"><div className="duration-select"><Clock3 size={16} /><select value={duration} disabled={running} onChange={e => setDuration(Number(e.target.value))}><option value="15">00 : 15</option><option value="30">00 : 30</option><option value="60">01 : 00</option><option value="120">02 : 00</option><option value="300">05 : 00</option><option value="600">10 : 00</option></select><ChevronDown size={15} /></div><span className="duration-hint">{text('Всего до', 'Up to')} <b>{(rate * duration).toLocaleString('ru-RU')}</b> {text('сеансов', 'sessions')}</span></div>
              <div className="scenario-box"><div className="scenario-top"><span className="scenario-tag">СЦЕНАРИЙ</span><span className="auto-tag"><span />АВТОМАТИЧЕСКИ</span></div><div className="flow"><div className="flow-step"><div className="flow-icon blue"><ArrowDown size={15} /></div><div><b>Получить слоты</b><small>GET /api/v1/slots</small></div></div><div className="flow-connector" /><div className="flow-step"><div className="flow-icon purple"><ArrowUpRight size={15} /></div><div><b>Забронировать слот</b><small>POST /api/v1/bookings</small></div></div><div className="flow-connector" /><div className="flow-step"><div className="flow-icon amber"><RotateCcw size={15} /></div><div><b>Отменить бронь</b><small>{text('1 из каждых 10 броней', '1 of every 10 bookings')}</small></div></div></div></div>
              <button className="advanced-toggle" onClick={() => setShowAdvanced(!showAdvanced)}><Settings2 size={15} />Параметры API<span>{showAdvanced ? 'Скрыть' : 'Изменить'}</span><ChevronDown size={14} className={showAdvanced ? 'rotate' : ''} /></button>
              {showAdvanced && <div className="advanced-fields"><label>API URL<input value={apiUrl} disabled={running} onChange={e => setApiUrl(e.target.value)} placeholder="http://127.0.0.1:8080" /></label><div className="advanced-two"><label>Регион<input value={region} disabled={running} onChange={e => setRegion(e.target.value)} /></label><label>Ресурс<input value={resource} disabled={running} onChange={e => setResource(e.target.value)} /></label></div><label>Customer ref<input value={customerRef} disabled={running} onChange={e => setCustomerRef(e.target.value)} /></label><label>EVM-кошелёк<input value={wallet} disabled={running} onChange={e => setWallet(e.target.value)} /></label><p>Все настройки сохраняются только в этом браузере и не передаются серверу.</p></div>}
              {notice && <div className="notice"><ShieldCheck size={16} />{notice}</div>}
              <div className="action-row">{running ? <button className="stop-button" onClick={() => stop()}><Square size={15} fill="currentColor" />Остановить тест</button> : <button className="run-button" onClick={start}><Play size={16} fill="currentColor" />Запустить тест<ArrowUpRight size={16} /></button>}<span className="action-secure"><ShieldCheck size={14} />Безопасно для локальной среды</span></div>
            </section>
            <section className="panel stats-panel"><div className="panel-heading stats-heading"><div className="heading-icon mint"><Activity size={17} /></div><div><h2>Результаты прогона</h2><p>Текущий тестовый запуск</p></div><span className={`live-badge ${running ? 'is-live' : ''}`}><span />{running ? 'В ПРОЦЕССЕ' : 'ОЖИДАНИЕ'}</span></div><div className="stats-grid"><Stat label="ЗАПУЩЕНО СЕАНСОВ" value={counts.sessions.toLocaleString('ru-RU')} accent="blue"/><Stat label="СЛОТОВ ПОЛУЧЕНО" value={counts.slots.toLocaleString('ru-RU')} accent="violet"/><Stat label="СОЗДАНО БРОНЕЙ" value={counts.booked.toLocaleString('ru-RU')} accent="mint"/><Stat label="ОТМЕНЕНО БРОНЕЙ" value={counts.cancelled.toLocaleString('ru-RU')} accent="amber"/></div><div className="progress-section"><div className="progress-label"><span>Прогресс теста</span><b>{elapsed}s <small>/ {duration}s</small></b></div><div className="progress-track"><div style={{ width: `${progress}%` }} /></div></div><div className="stats-footer"><span><span className="footer-dot green" />Успешно: <b>{counts.booked + counts.cancelled}</b></span><span><span className="footer-dot red" />Ошибки: <b>{counts.errors}</b></span><span className="footer-separator" /><span>Средняя скорость: <b>{elapsed ? (counts.sessions / Math.max(1, elapsed)).toFixed(1) : '0.0'} с/с</b></span></div></section>
          </div>
          <section className="panel activity-panel"><div className="activity-heading"><div><h2>Активность запросов</h2><p>Хронология операций теста</p></div><button className="activity-filter" onClick={() => setEvents(initialEvents)}><RotateCcw size={13} />Очистить</button></div><div className="activity-summary"><span><span className="summary-indicator live" />LIVE LOG</span><span className="summary-count">{events.filter(e => e.status !== 'ready').length} {text('событий', 'events')}</span></div><div className="event-list" ref={eventsRef}>{events.map((event, i) => <div className={`event ${event.status}`} key={`${event.time}-${i}`}><div className="event-rail"><span className="event-dot">{event.status === 'ok' ? <Check size={11} /> : event.status === 'error' ? <Ban size={11} /> : <Zap size={11} />}</span>{i < events.length - 1 && <span className="event-line" />}</div><div className="event-body"><div className="event-meta"><span className={`method ${event.method.toLowerCase()}`}>{event.method}</span><time>{event.time}</time></div><b>{event.title}</b><p>{event.detail}</p></div></div>)}</div><div className="activity-footer"><div className="connection-state"><span className="online-dot" />События обновляются в реальном времени</div><button className="external-link" title="OpenAPI спецификация"><ExternalLink size={15} /></button></div></section>
        </section>
        <footer className="page-footer"><span>DEBOOK <span className="footer-dot-small">·</span> LOAD LAB <span className="version">v1.0.0</span></span><span>Тестируйте ответственно <span className="footer-dot-small">·</span> <a href="http://localhost:8081" target="_blank" rel="noreferrer">OpenAPI документация <ExternalLink size={11} /></a></span></footer>
      </div>
    </main>
  </div>
}

function Stat({ label, value, accent }) { return <div className="stat-card"><div className={`stat-accent ${accent}`} /><span>{label}</span><b>{value}</b></div> }

export default App
