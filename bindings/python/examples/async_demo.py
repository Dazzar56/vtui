import asyncio
import vtui

def ui(u):
    with u.dialog(" Async Demo ", w=40):
        name = u.edit("&User:", "Alice")
        if u.button("&Submit"):
            u.message(" Welcome ", f"Hello async user: {name}")

async def main():
    await vtui.run_async(ui)

if __name__ == "__main__":
    asyncio.run(main())
