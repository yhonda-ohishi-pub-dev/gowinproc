import { useState } from 'react'

export function NotificationTest() {
  const [status, setStatus] = useState<string>('')
  const [isSubscribed, setIsSubscribed] = useState<boolean>(false)

  // プッシュ通知の権限をリクエスト
  const requestNotificationPermission = async () => {
    if (!('Notification' in window)) {
      setStatus('このブラウザは通知をサポートしていません')
      return false
    }

    const permission = await Notification.requestPermission()
    if (permission === 'granted') {
      setStatus('通知の権限が許可されました')
      return true
    } else {
      setStatus('通知の権限が拒否されました')
      return false
    }
  }

  // Service Workerを登録してプッシュ通知を購読
  const subscribeToPushNotifications = async () => {
    try {
      // 権限をリクエスト
      const hasPermission = await requestNotificationPermission()
      if (!hasPermission) return

      // Service Workerを登録
      const registration = await navigator.serviceWorker.register('/sw.js')
      setStatus('Service Worker登録完了')

      // プッシュマネージャーから購読を取得または作成
      let subscription = await registration.pushManager.getSubscription()

      if (!subscription) {
        // 新しい購読を作成（VAPIDキーは後で設定）
        subscription = await registration.pushManager.subscribe({
          userVisibleOnly: true,
          applicationServerKey: urlBase64ToUint8Array(
            'YOUR_VAPID_PUBLIC_KEY_HERE' // TODO: バックエンドから取得
          )
        })
        setStatus('プッシュ通知を購読しました')
      } else {
        setStatus('既にプッシュ通知を購読しています')
      }

      setIsSubscribed(true)
      console.log('Push subscription:', JSON.stringify(subscription))

      // TODO: サブスクリプションをバックエンドに送信
      // await fetch('/api/push/subscribe', {
      //   method: 'POST',
      //   headers: { 'Content-Type': 'application/json' },
      //   body: JSON.stringify(subscription)
      // })

    } catch (error) {
      console.error('プッシュ通知の購読に失敗:', error)
      setStatus(`エラー: ${error instanceof Error ? error.message : '不明なエラー'}`)
    }
  }

  // テスト通知を送信（ブラウザネイティブ通知）
  const sendTestNotification = async () => {
    try {
      const hasPermission = await requestNotificationPermission()
      if (!hasPermission) return

      new Notification('GoWinProc テスト通知', {
        body: 'これはテスト通知です',
        icon: '/vite.svg',
        badge: '/vite.svg',
        tag: 'test-notification',
        requireInteraction: false,
      })

      setStatus('テスト通知を送信しました')
    } catch (error) {
      console.error('通知の送信に失敗:', error)
      setStatus(`エラー: ${error instanceof Error ? error.message : '不明なエラー'}`)
    }
  }

  // プッシュ通知のテスト（バックエンド経由）
  const sendTestPushNotification = async () => {
    try {
      // TODO: バックエンドのプッシュ通知APIを呼び出す
      const response = await fetch('http://127.0.0.1:8080/api/push/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          title: 'GoWinProc プッシュ通知テスト',
          body: 'サーバーからのプッシュ通知です',
        })
      })

      if (response.ok) {
        setStatus('プッシュ通知をサーバーに送信しました')
      } else {
        setStatus(`エラー: ${response.statusText}`)
      }
    } catch (error) {
      console.error('プッシュ通知の送信に失敗:', error)
      setStatus(`エラー: ${error instanceof Error ? error.message : '不明なエラー'}`)
    }
  }

  return (
    <div style={{ padding: '20px' }}>
      <h2>通知テスト</h2>

      <div style={{ marginBottom: '20px' }}>
        <p>ステータス: {status || '待機中'}</p>
        <p>購読状態: {isSubscribed ? '購読済み' : '未購読'}</p>
      </div>

      <div style={{ display: 'flex', gap: '10px', flexDirection: 'column', maxWidth: '400px' }}>
        <button
          onClick={sendTestNotification}
          className="btn btn-primary"
          style={{ padding: '10px 20px' }}
        >
          🔔 テスト通知を送信（ブラウザ通知）
        </button>

        <button
          onClick={subscribeToPushNotifications}
          className="btn btn-secondary"
          style={{ padding: '10px 20px' }}
          disabled={isSubscribed}
        >
          📱 プッシュ通知を購読
        </button>

        <button
          onClick={sendTestPushNotification}
          className="btn btn-success"
          style={{ padding: '10px 20px' }}
          disabled={!isSubscribed}
        >
          📨 プッシュ通知をテスト（サーバー経由）
        </button>
      </div>

      <div style={{ marginTop: '30px', fontSize: '14px', color: '#666' }}>
        <h3>使い方:</h3>
        <ol>
          <li>「テスト通知を送信」: 即座にブラウザ通知を表示</li>
          <li>「プッシュ通知を購読」: Service Workerを登録してプッシュ通知を有効化</li>
          <li>「プッシュ通知をテスト」: サーバー経由でプッシュ通知を送信</li>
        </ol>
      </div>
    </div>
  )
}

// VAPID公開鍵をUint8Arrayに変換するヘルパー関数
function urlBase64ToUint8Array(base64String: string): Uint8Array {
  const padding = '='.repeat((4 - base64String.length % 4) % 4)
  const base64 = (base64String + padding)
    .replace(/\-/g, '+')
    .replace(/_/g, '/')

  const rawData = window.atob(base64)
  const outputArray = new Uint8Array(rawData.length)

  for (let i = 0; i < rawData.length; ++i) {
    outputArray[i] = rawData.charCodeAt(i)
  }
  return outputArray
}
