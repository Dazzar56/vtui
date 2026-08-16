import vtui

def ui(u):
    with u.dialog(" Hello vtui ", w=40):
        name = u.edit("&Name:", "Type here...")
        if u.button("&Ok"):
            u.message(" Result ", f"You typed:\n{name}")

if __name__ == "__main__":
    vtui.run(ui)
