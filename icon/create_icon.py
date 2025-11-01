from PIL import Image, ImageDraw

# Create 16x16 image with transparency
img = Image.new('RGBA', (16, 16), (0, 0, 0, 0))
draw = ImageDraw.Draw(img)

# Draw a green circle
draw.ellipse([3, 3, 13, 13], fill=(0, 200, 0, 255), outline=(0, 150, 0, 255))

# Save as ICO
img.save('icon.ico', format='ICO', sizes=[(16, 16)])
print("Icon created: icon.ico")
