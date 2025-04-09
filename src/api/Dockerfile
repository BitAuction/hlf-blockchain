# Use the official Python base image
FROM python:3.8.10

# Set the working directory in the container
WORKDIR /app

# Copy the source code to the working directory
COPY . /app

# Copy the requirements file
COPY requirements.txt .

# Install the required dependencies
RUN pip install --no-cache-dir -r requirements.txt
RUN pip install python-dotenv


# Expose the port your Flask API is running on
EXPOSE 5000
# Set the environment variable for Flask
ENV FLASK_APP=app.py
# Copy the .env file to the working directory
COPY .env .env

# Run the Flask app
CMD ["flask", "run", "--host=0.0.0.0"]