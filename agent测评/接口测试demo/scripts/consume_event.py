"""
脚本功能：
从 RabbitMQ 的 order.cancel.events 队列读取一条取消事件并打印 JSON 消息体。

启动命令：
在项目根目录运行 `.venv/bin/python scripts/consume_event.py`。
"""

import json

import pika

from app.config import load_settings


def main() -> None:
    """
    输入：无。
    输出：打印一条取消事件；队列为空时提示无消息。
    功能：手工验证取消事件是否已进入 RabbitMQ。
    """
    settings = load_settings()
    connection = pika.BlockingConnection(pika.URLParameters(settings.rabbitmq_url))
    try:
        channel = connection.channel()
        channel.queue_declare(queue="order.cancel.events", durable=True)
        method, _, body = channel.basic_get(queue="order.cancel.events", auto_ack=True)
        if method is None:
            print("队列为空，暂无可消费的取消事件")
            return
        print(json.dumps(json.loads(body), ensure_ascii=False, indent=2))
    finally:
        connection.close()


if __name__ == "__main__":
    main()
