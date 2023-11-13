# Use an official Python runtime as a parent image
FROM ubuntu:20.04

WORKDIR /

# Install any needed packages specified in requirements.txt
RUN apt-get update
RUN apt-get -y install lksctp-tools

# Make port 9999 available to the world outside this container
EXPOSE 9999

# Run app.py when the container launches
CMD ["sctp_darn", "-H 0.0.0.0 -P 9999 -l"]
