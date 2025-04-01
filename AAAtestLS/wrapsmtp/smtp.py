import asyncio  
from aiosmtpd.controller import Controller  

class CustomSMTP:  
    async def handle_DATA(self, server, session, envelope):  
        print('Message from:', envelope.mail_from)  
        print('Message for:', envelope.rcpt_tos)  
        print('Message data:\n', envelope.content.decode())  
        return '250 OK'  

async def main():  
    # Create an instance of your SMTP handler  
    handler = CustomSMTP()  
    
    # Create a controller to manage the server  
    controller = Controller(handler, hostname='127.0.0.1', port=8000)  
    
    # Start the SMTP server  
    controller.start()  

    try:  
        # Serve forever  
        await asyncio.Event().wait()  
    finally:  
        controller.stop()  

if __name__ == '__main__':  
    asyncio.run(main())  